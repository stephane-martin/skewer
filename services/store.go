package services

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/awnumar/memguard"
	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/store"
	"github.com/stephane-martin/skewer/store/dests"
	"github.com/stephane-martin/skewer/sys/kring"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/httpserver"
)

type storeServiceImpl struct {
	store            *store.MessageStore
	config           conf.BaseConfig
	logger           log15.Logger
	shutdownStore    context.CancelFunc
	shutdownCtx      context.Context
	cancelForwarders context.CancelFunc
	mu               *sync.Mutex
	ingestwg         *sync.WaitGroup
	pipe             *os.File
	status           bool
	secret           *memguard.LockedBuffer
	ring             kring.Ring
	forwarders       map[conf.DestinationType]store.Forwarder
	fmu              map[conf.DestinationType]*sync.Mutex
	fstatus          map[conf.DestinationType]bool
	fcancels         map[conf.DestinationType]context.CancelFunc
	confined         bool
}

// NewStoreService creates a StoreService.
// The StoreService is responsible to manage the lifecycle of the Store and the
// Kafka Forwarder that is fed by the Store.
func NewStoreService(env *base.ProviderEnv) (base.Provider, error) {
	if env.Pipe == nil {
		return nil, fmt.Errorf("The Store was not given a message pipe")
	}
	store.InitRegistry()
	dests.InitRegistry()
	impl := storeServiceImpl{
		ingestwg: &sync.WaitGroup{},
		mu:       &sync.Mutex{},
		status:   false,
		pipe:     env.Pipe,
		logger:   env.Logger,
		ring:     env.Ring,
		confined: env.Confined,
	}
	impl.fmu = map[conf.DestinationType]*sync.Mutex{}
	impl.fstatus = map[conf.DestinationType]bool{}
	impl.fcancels = map[conf.DestinationType]context.CancelFunc{}
	for _, desttype := range conf.Destinations {
		impl.fmu[desttype] = &sync.Mutex{}
	}
	impl.shutdownCtx, impl.shutdownStore = context.WithCancel(context.Background())

	if env.Profile {
		httpserver.ProfileServer(env.Binder)
	}
	return &impl, nil
}

func (s *storeServiceImpl) Type() base.Types {
	return base.Store
}

func (s *storeServiceImpl) SetConf(c conf.BaseConfig) {
}

func (s *storeServiceImpl) SetConfAndRestart(c conf.BaseConfig) ([]model.ListenerInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.doStop(nil)
	s.config = c
	if c.Main.EncryptIPC {
		secret, err := s.ring.GetBoxSecret()
		if err != nil {
			return nil, err
		}
		s.logger.Debug("The store receives messages from an encrypted pipe")
		s.secret = secret
	} else {
		s.secret = nil
	}
	return s.doStart(nil)
}

func (s *storeServiceImpl) FatalError() chan struct{} {
	// Errors return a channel to signal the Store fatal errors.
	// (Typically no space left on disk.)
	return s.store.Errors()
}

func (s *storeServiceImpl) Start() ([]model.ListenerInfo, error) {
	// unused
	return s.doStart(s.mu)
}

func (s *storeServiceImpl) create() error {
	destinations, err := s.config.Main.GetDestinations()
	if err != nil {
		return err
	}
	s.store, err = store.NewStore(s.shutdownCtx, s.config.Store, s.ring, destinations, s.confined, s.logger)
	if err != nil {
		return err
	}
	err = s.store.StoreAllSyslogConfigs(s.config)
	if err != nil {
		return err
	}
	// receive syslog messages on the pipe
	s.ingestwg.Add(1)
	go func() {
		defer s.ingestwg.Done()

		scanner := bufio.NewScanner(s.pipe)
		scanner.Split(utils.MakeDecryptSplit(s.secret))
		defer func() {
			if s.secret != nil {
				//s.secret.Destroy()
			}
		}()
		var err error

		for scanner.Scan() {
			message := model.FullMessage{}
			err = message.Unmarshal(scanner.Bytes())
			if err == nil {
				err, _ = s.store.Stash(message)
				if err != nil {
					s.logger.Error("Error pushing message to the store queue", "error", err)
					go func() { s.Shutdown() }()
					return
				}
			} else {
				s.logger.Error("Unexpected error decoding message from the Store pipe", "error", err)
				go func() { s.Shutdown() }()
				return
			}
		}

		err = scanner.Err()
		if utils.IsFileClosed(err) {
			s.logger.Debug("Stopped to read the ingestion store pipe")
		} else if err != nil {
			s.logger.Warn("Unexpected error decoding message from the Store pipe", "error", err)
		}
	}()
	return nil
}

func (s *storeServiceImpl) startAllForwarders(dests conf.DestinationType) {
	// returns immediately
	var gforwarderCtx context.Context
	gforwarderCtx, s.cancelForwarders = context.WithCancel(s.shutdownCtx)
	s.forwarders = map[conf.DestinationType]store.Forwarder{}
	for _, dtype := range dests.Iterate() {
		desttype := dtype
		s.startForwarder(gforwarderCtx, desttype)
	}
}

func (s *storeServiceImpl) stopAllForwarders() {
	if s.cancelForwarders != nil {
		s.cancelForwarders()
		s.cancelForwarders = nil
		var wg sync.WaitGroup
		for _, dtype := range conf.Destinations {
			wg.Add(1)
			go func(desttype conf.DestinationType) {
				s.stopForwarder(desttype)
				wg.Done()
			}(dtype)
		}
		wg.Wait()
	}
}

func (s *storeServiceImpl) stopForwarder(desttype conf.DestinationType) {
	s.fmu[desttype].Lock()
	defer s.fmu[desttype].Unlock()
	if !s.fstatus[desttype] {
		return
	}
	s.fcancels[desttype]()
	s.fcancels[desttype] = nil
	s.forwarders[desttype].WaitFinished()
	s.forwarders[desttype] = nil
	s.fstatus[desttype] = false
}

func (s *storeServiceImpl) startForwarder(gforwarderCtx context.Context, desttype conf.DestinationType) {
	// returns immediately
	s.fmu[desttype].Lock()
	if s.fstatus[desttype] {
		s.fmu[desttype].Unlock()
		return
	}
	s.logger.Info("Starting forwarder", "type", conf.DestinationNames[desttype])
	s.fstatus[desttype] = true

	ctx, cancel := context.WithCancel(gforwarderCtx)
	forwarder := store.NewForwarder(desttype, s.store, s.config, s.logger)
	s.forwarders[desttype] = forwarder
	s.fcancels[desttype] = cancel
	s.fmu[desttype].Unlock()

	// start forwarding message
	forwarder.Forward(ctx)

	// monitor for remote connection errors
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-forwarder.Fatal():
			// when the kafka forwarder signals an error,
			// the StoreService stops the forwarder,
			// waits 10 seconds, and then restarts the
			// forwarder.
			// but at the same time the main process can
			// decide to stop/start the StoreService (for
			// instance because it received a SIGHUP).
			// that's why we need the mutex stuff
			s.logger.Warn("The forwarder reported a connection error", "type", conf.DestinationNames[desttype])
			s.stopForwarder(desttype)
			select {
			case <-gforwarderCtx.Done():
			case <-time.After(10 * time.Second):
				s.startForwarder(gforwarderCtx, desttype)
			}
		}
	}()

}

func (s *storeServiceImpl) doStart(mu *sync.Mutex) ([]model.ListenerInfo, error) {
	infos := []model.ListenerInfo{}
	if mu != nil {
		mu.Lock()
		defer mu.Unlock()
	}

	select {
	case <-s.shutdownCtx.Done():
		// the Store is shutdown
		return infos, nil
	default:
	}

	if s.status {
		// already started
		return infos, nil
	}

	destinations, err := s.config.Main.GetDestinations()
	if err != nil {
		s.logger.Error("Error parsing destinations (should not happen!!!)", "error", err)
		return nil, err
	}

	// create the store if needed
	if s.store == nil {
		err = s.create()
		if err != nil {
			return infos, err
		}
	}

	// refresh destinations
	s.store.SetDestinations(destinations)
	s.status = true

	// create and start the forwarders
	s.startAllForwarders(destinations)

	return infos, nil
}

// Stop stops the Kafka forwarder
func (s *storeServiceImpl) Stop() {
	s.doStop(s.mu)
}

func (s *storeServiceImpl) doStop(mu *sync.Mutex) {
	if mu != nil {
		mu.Lock()
		defer mu.Unlock()
	}

	select {
	case <-s.shutdownCtx.Done():
		// the Store is shutdown
		return
	default:
	}

	if !s.status {
		// already stopped
		return
	}

	// stop the kafka forwarders
	s.stopAllForwarders()
	s.status = false
}

// Shutdown stops the kafka forwarder and shutdowns the Store
func (s *storeServiceImpl) Shutdown() {
	select {
	case <-s.shutdownCtx.Done():
		// the Store is already shutdown
		return
	default:
	}
	s.Stop()
	s.ingestwg.Wait() // wait until we are done ingesting new messages
	s.shutdownStore()
	s.logger.Debug("Store service waits for end of store goroutines")
	s.store.WaitFinished()
	_ = s.pipe.Close()
}

// Gather returns the metrics for the Store and the Kafka forwarder
func (s *storeServiceImpl) Gather() ([]*dto.MetricFamily, error) {
	var couple prometheus.Gatherers = []prometheus.Gatherer{store.Registry, dests.Registry}
	return couple.Gather()
}

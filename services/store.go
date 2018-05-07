package services

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/awnumar/memguard"
	"github.com/gogo/protobuf/proto"
	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	circuit "github.com/rubyist/circuitbreaker"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/store"
	"github.com/stephane-martin/skewer/store/dests"
	"github.com/stephane-martin/skewer/sys/binder"
	"github.com/stephane-martin/skewer/sys/kring"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/stephane-martin/skewer/utils/httpserver"
)

type storeServiceImpl struct {
	store            *store.MessageStore
	config           conf.BaseConfig
	logger           log15.Logger
	binder           binder.Client
	shutdownStore    context.CancelFunc
	shutdownCtx      context.Context
	pipeCtx          context.Context
	cancelForwarders context.CancelFunc
	cancelPipe       context.CancelFunc
	fwdersWg         sync.WaitGroup
	mu               sync.Mutex
	ingestwg         sync.WaitGroup
	pipe             *os.File
	status           bool
	secret           *memguard.LockedBuffer
	ring             kring.Ring
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
		status:   false,
		pipe:     env.Pipe,
		logger:   env.Logger,
		binder:   env.Binder,
		ring:     env.Ring,
		confined: env.Confined,
	}
	impl.shutdownCtx, impl.shutdownStore = context.WithCancel(context.Background())
	impl.pipeCtx, impl.cancelPipe = context.WithCancel(impl.shutdownCtx)

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
	return s.doStart()
}

func (s *storeServiceImpl) FatalError() chan struct{} {
	// Errors return a channel to signal the Store fatal errors.
	// (Typically no space left on disk.)
	return s.store.Errors()
}

func (s *storeServiceImpl) Start() ([]model.ListenerInfo, error) {
	// unused
	return nil, nil
}

func (s *storeServiceImpl) create() (err error) {
	var destinations conf.DestinationType
	destinations, err = s.config.Main.GetDestinations()
	if err != nil {
		return eerrors.Wrap(err, "Error getting destinations")
	}
	s.store, err = store.NewStore(s.shutdownCtx, s.config.Store, s.ring, destinations, s.confined, s.logger)
	if err != nil {
		return eerrors.Wrap(err, "Error creating the Store")
	}
	err = s.store.StoreAllSyslogConfigs(s.config)
	if err != nil {
		return eerrors.Wrap(err, "Error storing configurations in store")
	}
	// receive syslog messages on the pipe
	s.ingestwg.Add(1)
	go func() {
		defer func() {
			if e := eerrors.Err(recover()); e != nil {
				err := eerrors.Wrap(e, "Scanner panicked in store service")
				s.logger.Error(err.Error())
			}
			s.ingestwg.Done()
		}()

		scanner := utils.NewWrappedScanner(s.pipeCtx, bufio.NewScanner(s.pipe))
		scanner.Split(utils.MakeDecryptSplit(s.secret))
		scanner.Buffer(make([]byte, 0, 65536), 65536)

		var err error
		protobuff := proto.NewBuffer(make([]byte, 0, 4096))

		for {
			for scanner.Scan() {
				msgBytes := scanner.Bytes()
				protobuff.SetBuf(msgBytes)
				message, err := model.FromBuf(protobuff) // we need to parse to get the message uid
				if err != nil {
					model.FullFree(message)
					s.logger.Error("Unexpected error decoding message from the Store pipe", "error", err)
					go func() { s.Shutdown() }()
					return
				}
				uid := message.Uid
				model.FullFree(message)
				err = s.store.Stash(uid, string(msgBytes))
				if err != nil {
					s.logger.Error("Error pushing message to the store queue", "error", err)
					go func() { s.Shutdown() }()
					return
				}
			}

			err = scanner.Err()
			if eerrors.IsTimeout(err) {
				continue
			}
			if err == nil || eerrors.HasFileClosed(err) {
				s.logger.Debug("Stopped to read the ingestion store pipe")
				return
			} else if err != nil {
				s.logger.Warn("Unexpected error decoding message from the Store pipe", "error", err)
			}
		}
	}()
	return nil
}

func (s *storeServiceImpl) startAllForwarders(dests conf.DestinationType) {
	// returns immediately
	var gforwarderCtx context.Context
	gforwarderCtx, s.cancelForwarders = context.WithCancel(s.shutdownCtx)
	for _, dtype := range dests.Iterate() {
		desttype := dtype
		s.fwdersWg.Add(1)
		go s.startForwarder(gforwarderCtx, desttype)
	}
}

func (s *storeServiceImpl) stopAllForwarders() {
	if s.cancelForwarders != nil {
		s.cancelForwarders()
	}
	s.fwdersWg.Wait()
}

func (s *storeServiceImpl) startForwarder(ctx context.Context, desttype conf.DestinationType) {
	defer s.fwdersWg.Done()
	s.logger.Info("Starting forwarder", "type", conf.DestinationNames[desttype])
	forwarder := store.NewForwarder(desttype, s.store, s.config, s.logger, s.binder)
	cb := circuit.NewConsecutiveBreaker(3)

	for {
		fctx, fcancel := context.WithCancel(ctx)
		// we use a circuit breaker to prevent from trying to create the destination too often
		err := cb.CallContext(
			fctx,
			func() error { return forwarder.CreateDestination(fctx) },
			time.Minute,
		)
		if err != nil {
			fcancel()
			if err != circuit.ErrBreakerOpen {
				s.logger.Error("Forwarder faced an error when creating destination", "dest", desttype, "error", err)
			}
		} else {
			// destination was successfully created
			err = forwarder.Forward(fctx)
			fcancel()
			if err == nil {
				return
			}
			s.logger.Error("Forwarder error", "dest", desttype, "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func (s *storeServiceImpl) doStart() ([]model.ListenerInfo, error) {
	infos := []model.ListenerInfo{}

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
	s.doStop(&s.mu)
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

// Shutdown stops the forwarders and shutdowns the Store
func (s *storeServiceImpl) Shutdown() {
	select {
	case <-s.shutdownCtx.Done():
		// the Store is already shutdown
		return
	default:
	}
	s.Stop()
	s.cancelPipe()
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

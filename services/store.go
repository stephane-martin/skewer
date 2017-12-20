package services

import (
	"bufio"
	"context"
	"io"
	"os"
	"sync"
	"time"

	"github.com/awnumar/memguard"
	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/store"
	"github.com/stephane-martin/skewer/sys/kring"
	"github.com/stephane-martin/skewer/utils"
)

type storeServiceImpl struct {
	st              *store.MessageStore
	config          conf.BaseConfig
	logger          log15.Logger
	shutdownStore   context.CancelFunc
	shutdownCtx     context.Context
	cancelForwarder context.CancelFunc
	forwarder       store.Forwarder
	mu              *sync.Mutex
	ingestwg        *sync.WaitGroup
	pipe            *os.File
	status          bool
	test            bool
	secret          *memguard.LockedBuffer
	ring            kring.Ring
}

// NewStoreService creates a StoreService.
// The StoreService is responsible to manage the lifecycle of the Store and the
// Kafka Forwarder that is fed by the Store.
func NewStoreService(l log15.Logger, ring kring.Ring, pipe *os.File) Provider {
	if pipe == nil {
		l.Crit("The Store was not given a message pipe")
		return nil
	}
	impl := &storeServiceImpl{
		ingestwg: &sync.WaitGroup{},
		mu:       &sync.Mutex{},
		status:   false,
		pipe:     pipe,
		logger:   l,
		ring:     ring,
	}
	impl.shutdownCtx, impl.shutdownStore = context.WithCancel(context.Background())
	return impl
}

func (s *storeServiceImpl) SetConfAndRestart(c conf.BaseConfig, test bool) ([]model.ListenerInfo, error) {
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
	return s.doStart(test, nil)
}

func (s *storeServiceImpl) FatalError() chan struct{} {
	// Errors return a channel to signal the Store fatal errors.
	// (Typically no space left on disk.)
	return s.st.Errors()
}

// Start starts the Kafka forwarder
func (s *storeServiceImpl) Start(test bool) ([]model.ListenerInfo, error) {
	return s.doStart(test, s.mu)
}

func (s *storeServiceImpl) doStart(test bool, mu *sync.Mutex) ([]model.ListenerInfo, error) {
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

	// create the store if needed
	if s.st == nil {
		s.test = test
		destinations, err := s.config.Main.GetDestinations()
		if err != nil {
			return infos, err
		}
		s.st, err = store.NewStore(s.shutdownCtx, s.config.Store, s.ring, destinations, s.logger)
		if err != nil {
			return infos, err
		}
		err = s.st.StoreAllSyslogConfigs(s.config)
		if err != nil {
			return infos, err
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
				_, err = message.UnmarshalMsg(scanner.Bytes())
				if err == nil {
					err, _ = s.st.Stash(message)
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
			if err == io.EOF || err == io.ErrClosedPipe || err == io.ErrUnexpectedEOF {
				s.logger.Debug("Stopped to read the ingestion store pipe")
			} else if err != nil {
				s.logger.Warn("Unexpected error decoding message from the Store pipe", "error", err)
			}
		}()
	}

	destinations, err := s.config.Main.GetDestinations()
	if err != nil {
		s.logger.Warn("Error parsing destinations (should not happen!!!)", "error", err)
	} else {
		// refresh destinations
		s.st.SetDestinations(destinations)
	}

	// create and start the forwarder
	var forwarderCtx context.Context
	forwarderCtx, s.cancelForwarder = context.WithCancel(context.Background())
	s.forwarder = store.NewForwarder(test, s.logger)
	s.forwarder.Forward(forwarderCtx, s.st, s.config)
	s.status = true

	go func() {
		done := forwarderCtx.Done()
		shutdown := s.shutdownCtx.Done()
		fwderFatalError := s.forwarder.Fatal()
		// monitor for remote connection errors
		select {
		case <-done:
			// the main process has stopped the forwarder
			// we don't need to monitor for errors anymore
			return
		case <-shutdown:
			// the Store was shutdown
			return
		case <-fwderFatalError:
			// when the kafka forwarder signals an error,
			// the StoreService stops the forwarder,
			// waits 10 seconds, and then restarts the
			// forwarder.
			// but at the same time the main process can
			// decide to stop/start the StoreService (for
			// instance because it received a SIGHUP).
			// that's why we need the mutex stuff in Start(),
			// Stop(), SetConfAndRestart() methods.
			tryStart := func() error {
				select {
				case <-s.shutdownCtx.Done():
					return nil
				case <-time.After(10 * time.Second):
					_, err := s.Start(test)
					return err
				}
			}
			s.logger.Warn("The forwarder reported a connection error")
			s.Stop()
			for tryStart() != nil {
				s.logger.Warn("Failed to start the forwarder", "error", "err")
			}
		}
	}()
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

	// stop the kafka forwarder
	s.cancelForwarder()
	s.forwarder.WaitFinished()
	s.logger.Debug("Forwarder has stopped")
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
	s.st.WaitFinished()
	_ = s.pipe.Close()
}

// Gather returns the metrics for the Store and the Kafka forwarder
func (s *storeServiceImpl) Gather() ([]*dto.MetricFamily, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status {
		// forwarder is started
		var couple prometheus.Gatherers = []prometheus.Gatherer{s.st, s.forwarder}
		return couple.Gather()
	} else if s.st == nil {
		return nil, nil
	} else {
		return s.st.Gather()
	}
}

package services

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/store"
	"github.com/tinylib/msgp/msgp"
)

type storeServiceImpl struct {
	st              store.Store
	config          conf.BaseConfig
	logger          log15.Logger
	shutdownStore   context.CancelFunc
	shutdownCtx     context.Context
	cancelForwarder context.CancelFunc
	forwarder       store.Forwarder
	mu              *sync.Mutex
	reader          *msgp.Reader
	ingestwg        *sync.WaitGroup
	pipe            *os.File
	status          bool
	test            bool
}

// NewStoreService creates a StoreService.
// The StoreService is responsible to manage the lifecycle of the Store and the
// Kafka Forwarder that is fed by the Store.
func NewStoreService(l log15.Logger, pipe *os.File) StoreService {
	if pipe == nil {
		l.Crit("The Store was not given a message pipe")
		return nil
	}
	impl := &storeServiceImpl{
		reader:   msgp.NewReader(pipe),
		ingestwg: &sync.WaitGroup{},
		mu:       &sync.Mutex{},
		status:   false,
		pipe:     pipe,
		logger:   l,
	}
	impl.shutdownCtx, impl.shutdownStore = context.WithCancel(context.Background())
	return impl
}

func (s *storeServiceImpl) SetConfAndRestart(c conf.BaseConfig, test bool) ([]model.ListenerInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.doStop(nil)
	s.config = c
	return s.doStart(test, nil)
}

// Errors return a channel to signal the Store fatal errors.
// Typically no space left on disk.
func (s *storeServiceImpl) Errors() chan struct{} {
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
		var err error
		s.test = test

		s.st, err = store.NewStore(s.shutdownCtx, s.config.Store, s.logger)
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
			var err error
			var message model.TcpUdpParsedMessage
			for {
				message = model.TcpUdpParsedMessage{}
				err = message.DecodeMsg(s.reader)
				if err == nil {
					s.st.Stash(message)
				} else if err == io.EOF || err == io.ErrClosedPipe || err == io.ErrUnexpectedEOF {
					return
				} else {
					fmt.Fprintln(os.Stderr, "ZOOOOG", err)
				}

			}
		}()
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
			s.logger.Warn("The forwarder reported a connection error")
			s.Stop()
			select {
			case <-s.shutdownCtx.Done():
				return
			case <-time.After(10 * time.Second):
				s.Start(test)
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
	s.st.WaitFinished()
	s.pipe.Close()
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

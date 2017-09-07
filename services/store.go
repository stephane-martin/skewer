package services

import (
	"context"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/store"
)

type StoreServiceImpl struct {
	st              store.Store
	config          conf.BaseConfig
	logger          log15.Logger
	shutdownStore   context.CancelFunc
	shutdownCtx     context.Context
	cancelForwarder context.CancelFunc
	forwarder       store.Forwarder
	mu              *sync.Mutex
	status          bool
	test            bool
}

func NewStoreService(l log15.Logger) StoreService {
	impl := &StoreServiceImpl{
		logger: l,
		mu:     &sync.Mutex{},
		status: false,
	}
	impl.shutdownCtx, impl.shutdownStore = context.WithCancel(context.Background())
	return impl
}

func (s *StoreServiceImpl) SetConf(c conf.BaseConfig, test bool) ([]*model.ListenerInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.doStop(nil)
	s.config = c
	return s.doStart(test, nil)
}

func (s *StoreServiceImpl) Errors() chan struct{} {
	return s.st.Errors()
}

func (s *StoreServiceImpl) Start(test bool) ([]*model.ListenerInfo, error) {
	return s.doStart(test, s.mu)
}

func (s *StoreServiceImpl) doStart(test bool, mu *sync.Mutex) ([]*model.ListenerInfo, error) {
	if mu != nil {
		mu.Lock()
		defer mu.Unlock()
	}

	select {
	case <-s.shutdownCtx.Done():
		// the Store is shutdown
		return nil, nil
	default:
	}

	if s.status {
		// already started
		return nil, nil
	}

	// create the store if needed
	if s.st == nil {
		var err error
		s.test = test

		s.st, err = store.NewStore(s.shutdownCtx, s.config.Store, s.logger)
		if err != nil {
			return nil, err
		}
		err = s.st.StoreAllSyslogConfigs(s.config)
		if err != nil {
			return nil, err
		}
	}
	// create and start the kafka forwarder
	var forwarderCtx context.Context
	forwarderCtx, s.cancelForwarder = context.WithCancel(context.Background())
	s.forwarder = store.NewForwarder(test, s.logger)
	errorChan := s.forwarder.ErrorChan()
	s.forwarder.Forward(forwarderCtx, s.st, s.config.Kafka)
	s.status = true

	go func() {
		// monitor for kafka connection errors
		select {
		case <-forwarderCtx.Done():
			// the main process has stopped the forwarder
			// we don't need to monitor for errors anymore
			return
		case <-s.shutdownCtx.Done():
			// the Store was shutdown
			return
		case <-errorChan:
			s.Stop()
			// try to restart the forwarder after 10 seconds
			select {
			case <-s.shutdownCtx.Done():
				return
			case <-time.After(10 * time.Second):
				s.Start(test)
			}

		}
	}()
	return nil, nil
}

func (s *StoreServiceImpl) Stop() {
	s.doStop(s.mu)
}

func (s *StoreServiceImpl) doStop(mu *sync.Mutex) {
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

func (s *StoreServiceImpl) Shutdown() {
	// stop the kafka forwarder and shutdown the store
	select {
	case <-s.shutdownCtx.Done():
		// the Store is already shutdown
		return
	default:
	}

	s.Stop()
	s.shutdownStore()
	s.st.WaitFinished()
}

func (s *StoreServiceImpl) WaitClosed() {
	// wait that the kafka forwarder has stopped
	s.forwarder.WaitFinished()
}

func (s *StoreServiceImpl) Gather() ([]*dto.MetricFamily, error) {
	// gather metrics from the store and from the forwarder
	return nil, nil
}

func (s *StoreServiceImpl) Stash(m *model.TcpUdpParsedMessage) {
	// store a message in the store
	s.st.Stash(m)
}

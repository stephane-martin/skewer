package store

import (
	"context"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/javascript"
	"github.com/stephane-martin/skewer/model"
)

/*
type forwarderMetrics struct {
	KafkaConnectionErrorCounter prometheus.Counter
}
*/

type storeCallback func(uid ulid.ULID, dest conf.DestinationType)

type forwarderImpl struct {
	messageFilterCounter *prometheus.CounterVec
	logger               log15.Logger
	fatalChan            chan struct{}
	wg                   *sync.WaitGroup
	test                 bool
	registry             *prometheus.Registry
	dests                map[conf.DestinationType]Destination
	once                 sync.Once
}

func NewForwarder(test bool, logger log15.Logger) (fwder Forwarder) {
	f := forwarderImpl{
		test:     test,
		logger:   logger.New("class", "kafkaForwarder"),
		registry: prometheus.NewRegistry(),
		dests:    map[conf.DestinationType]Destination{},
	}

	f.messageFilterCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_fwder_messages_filtering_total",
			Help: "number of filtered messages by status",
		},
		[]string{"status", "client"},
	)

	f.registry.MustRegister(
		f.messageFilterCounter,
	)
	f.fatalChan = make(chan struct{})
	f.wg = &sync.WaitGroup{}
	return &f
}

func (fwder *forwarderImpl) Gather() ([]*dto.MetricFamily, error) {
	if len(fwder.dests) == 0 {
		return fwder.registry.Gather()
	}
	gatherers := prometheus.Gatherers{fwder.registry}
	for _, d := range fwder.dests {
		gatherers = append(gatherers, d)
	}
	return gatherers.Gather()
}

func (fwder *forwarderImpl) Fatal() chan struct{} {
	return fwder.fatalChan
}

func (fwder *forwarderImpl) WaitFinished() {
	fwder.wg.Wait()
}

func (fwder *forwarderImpl) Forward(ctx context.Context, from Store, bc conf.BaseConfig) {
	fwder.fatalChan = make(chan struct{})
	fwder.dests = map[conf.DestinationType]Destination{}
	dests := from.Destinations()
	for _, d := range dests {
		fwder.wg.Add(1)
		go fwder.forwardByDest(ctx, from, bc, d)
	}
}

func (fwder *forwarderImpl) forwardByDest(ctx context.Context, store Store, bc conf.BaseConfig, desttype conf.DestinationType) {
	var err error
	var dest Destination
	defer fwder.wg.Done()

	dest, err = NewDestination(ctx, desttype, bc, store.ACK, store.NACK, store.PermError, fwder.logger)
	if err != nil {
		fwder.logger.Error("Error setting up the destination", "error", err)
		close(fwder.fatalChan)
		return
	}

	defer dest.Close()

	// listen for destination fatal errors
	fwder.wg.Add(1)
	go func() {
		defer fwder.wg.Done()
		select {
		case <-ctx.Done():
			return
		case <-dest.Fatal():
			fwder.once.Do(func() {
				close(fwder.fatalChan)
			})
		}
	}()
	fwder.forwardMessagesByDest(ctx, store, dest, desttype)
}

func (fwder *forwarderImpl) forwardMessagesByDest(ctx context.Context, store Store, dest Destination, desttype conf.DestinationType) {
	jsenvs := map[ulid.ULID]*javascript.Environment{}
	done := ctx.Done()
	outputs := store.Outputs(desttype)
	var more bool
	var message *model.FullMessage
	var err error

	for {
		select {
		case <-done:
			return
		case message, more = <-outputs:
			if !more {
				return
			}
			if message != nil {
				err = fwder.forwardMessage(message, jsenvs, store, dest, desttype)
				store.ReleaseMsg(message)
				if err != nil {
					fwder.logger.Warn("Error forwarding message", "error", err, "uid", ulid.ULID(message.Uid).String())
				}
			}
		}
	}
}

func (fwder *forwarderImpl) forwardMessage(message *model.FullMessage, jsenvs map[ulid.ULID]*javascript.Environment, store Store, dest Destination, desttype conf.DestinationType) (err error) {
	var errs []error
	var config *conf.SyslogConfig
	var topic, partitionKey string
	var partitionNumber int32
	var filterResult javascript.FilterResult

	env, ok := jsenvs[message.ConfId]
	if !ok {
		// create the environement for the javascript virtual machine
		config, err = store.GetSyslogConfig(message.ConfId)
		if err != nil {
			fwder.logger.Warn("Could not find the stored configuration for a message", "confId", message.ConfId, "msgId", message.Uid)
			store.PermError(message.Uid, desttype)
			return err
		}
		jsenvs[message.ConfId] = javascript.NewFilterEnvironment(
			config.FilterFunc,
			config.TopicFunc,
			config.TopicTmpl,
			config.PartitionFunc,
			config.PartitionTmpl,
			config.PartitionNumberFunc,
			fwder.logger,
		)
		env = jsenvs[message.ConfId]
	}

	topic, errs = env.Topic(message.Parsed.Fields)
	for _, err = range errs {
		fwder.logger.Info("Error calculating topic", "error", err, "uid", message.Uid)
	}
	if len(topic) == 0 {
		topic = "default-topic"
	}
	partitionKey, errs = env.PartitionKey(message.Parsed.Fields)
	for _, err := range errs {
		fwder.logger.Info("Error calculating the partition key", "error", err, "uid", message.Uid)
	}
	partitionNumber, errs = env.PartitionNumber(message.Parsed.Fields)
	for _, err := range errs {
		fwder.logger.Info("Error calculating the partition number", "error", err, "uid", message.Uid)
	}

	filterResult, err = env.FilterMessage(&message.Parsed.Fields)

	switch filterResult {
	case javascript.DROPPED:
		store.ACK(message.Uid, desttype)
		fwder.messageFilterCounter.WithLabelValues("dropped", message.Parsed.Client).Inc()
		return
	case javascript.REJECTED:
		fwder.messageFilterCounter.WithLabelValues("rejected", message.Parsed.Client).Inc()
		store.NACK(message.Uid, desttype)
		return
	case javascript.PASS:
		fwder.messageFilterCounter.WithLabelValues("passing", message.Parsed.Client).Inc()
	default:
		store.PermError(message.Uid, desttype)
		fwder.logger.Warn("Error happened processing message", "uid", message.Uid, "error", err)
		fwder.messageFilterCounter.WithLabelValues("unknown", message.Parsed.Client).Inc()
		return err
	}

	return dest.Send(*message, partitionKey, partitionNumber, topic)

}

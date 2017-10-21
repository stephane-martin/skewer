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

type storeCallback func(uid ulid.ULID)

type forwarderImpl struct {
	messageFilterCounter *prometheus.CounterVec
	logger               log15.Logger
	fatalChan            chan struct{}
	wg                   *sync.WaitGroup
	test                 bool
	registry             *prometheus.Registry
	dest                 Destination
}

func NewForwarder(test bool, logger log15.Logger) (fwder Forwarder) {
	f := forwarderImpl{
		test:     test,
		logger:   logger.New("class", "kafkaForwarder"),
		registry: prometheus.NewRegistry(),
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
	if fwder.dest == nil {
		return fwder.registry.Gather()
	}
	return prometheus.Gatherers{fwder.registry, fwder.dest}.Gather()
}

func (fwder *forwarderImpl) Fatal() chan struct{} {
	return fwder.fatalChan
}

func (fwder *forwarderImpl) WaitFinished() {
	fwder.wg.Wait()
}

func (fwder *forwarderImpl) Forward(ctx context.Context, from Store, bc conf.BaseConfig) {
	fwder.fatalChan = make(chan struct{})
	fwder.wg.Add(1)
	go fwder.doForward(ctx, from, bc)
}

func (fwder *forwarderImpl) doForward(ctx context.Context, store Store, bc conf.BaseConfig) {
	defer fwder.wg.Done()
	if fwder.test {
		fwder.dest = nil
	} else {
		fwder.dest = NewDestination(ctx, bc.Main.Dest, bc, store.ACK, store.NACK, store.PermError, fwder.logger)
		if fwder.dest == nil {
			return
		}

		// listen for destination fatal errors
		fwder.wg.Add(1)
		go func() {
			defer fwder.wg.Done()
			select {
			case <-ctx.Done():
				return
			case <-fwder.dest.Fatal():
				close(fwder.fatalChan)
			}
		}()
	}

	fwder.wg.Add(1)
	go fwder.forwardMessages(ctx, store)

}

func (fwder *forwarderImpl) forwardMessages(ctx context.Context, store Store) {
	defer func() {
		// we close the destination only after we have stopped to forward messages
		if fwder.dest != nil {
			fwder.dest.Close()
		}
		fwder.wg.Done()
	}()

	jsenvs := map[ulid.ULID]*javascript.Environment{}
	done := ctx.Done()
	var more bool
	var message *model.TcpUdpParsedMessage
	var topic string
	var partitionKey string
	var partitionNumber int32
	var errs []error
	var err error
	var filterResult javascript.FilterResult
	var uid ulid.ULID

ForOutputs:
	for {
		select {
		case <-done:
			return
		case message, more = <-store.Outputs():
			if !more {
				return
			}
			uid = message.Uid
			env, ok := jsenvs[message.ConfId]
			if !ok {
				// create the environement for the javascript virtual machine
				config, err := store.GetSyslogConfig(message.ConfId)
				if err != nil {
					fwder.logger.Warn("Could not find the stored configuration for a message", "confId", message.ConfId, "msgId", message.Uid)
					store.PermError(uid)
					continue ForOutputs
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
				store.ACK(uid)
				fwder.messageFilterCounter.WithLabelValues("dropped", message.Parsed.Client).Inc()
				continue ForOutputs
			case javascript.REJECTED:
				fwder.messageFilterCounter.WithLabelValues("rejected", message.Parsed.Client).Inc()
				store.NACK(uid)
				continue ForOutputs
			case javascript.PASS:
				fwder.messageFilterCounter.WithLabelValues("passing", message.Parsed.Client).Inc()
			default:
				store.PermError(uid)
				fwder.logger.Warn("Error happened processing message", "uid", message.Uid, "error", err)
				fwder.messageFilterCounter.WithLabelValues("unknown", message.Parsed.Client).Inc()
				continue ForOutputs
			}

			if fwder.dest == nil {
				fwder.logger.Debug(
					"New message to be forwarded",
					"pkey", partitionKey,
					"topic", topic,
					"msgid", ulid.ULID(message.Uid).String(),
					"msg", message.Parsed.Fields.String(),
				)
				store.ACK(uid)
			} else {
				err := fwder.dest.Send(message, partitionKey, partitionNumber, topic)
				if err != nil {
					fwder.logger.Warn("Error forwarding message", "error", err, "uid", ulid.ULID(message.Uid).String())
				}
			}
		}
	}
}

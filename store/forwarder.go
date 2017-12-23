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
	"github.com/stephane-martin/skewer/store/dests"
)

/*
type forwarderMetrics struct {
	KafkaConnectionErrorCounter prometheus.Counter
}
*/

type fwderImpl struct {
	messageFilterCounter *prometheus.CounterVec
	logger               log15.Logger
	wg                   *sync.WaitGroup
	registry             *prometheus.Registry
	once                 sync.Once
	store                Store
	conf                 conf.BaseConfig
	desttype             conf.DestinationType
	fatalChan            chan struct{}
}

func NewForwarder(desttype conf.DestinationType, st Store, bc conf.BaseConfig, logger log15.Logger) (fwder Forwarder) {
	f := fwderImpl{
		logger:    logger.New("class", "forwarder"),
		registry:  prometheus.NewRegistry(),
		store:     st,
		conf:      bc,
		desttype:  desttype,
		fatalChan: make(chan struct{}),
	}

	f.messageFilterCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_fwder_messages_filtering_total",
			Help: "number of filtered messages by status",
		},
		[]string{"status", "client", "destination"},
	)

	f.registry.MustRegister(
		f.messageFilterCounter,
	)
	f.fatalChan = make(chan struct{})
	f.wg = &sync.WaitGroup{}
	return &f
}

func (fwder *fwderImpl) Gather() ([]*dto.MetricFamily, error) {
	return dests.DestsRegistry.Gather()
}

func (fwder *fwderImpl) Fatal() chan struct{} {
	return fwder.fatalChan
}

func (fwder *fwderImpl) dofatal() {
	fwder.once.Do(func() { close(fwder.fatalChan) })
}

func (fwder *fwderImpl) WaitFinished() {
	fwder.wg.Wait()
}

func (fwder *fwderImpl) Forward(ctx context.Context) {
	fwder.wg.Add(1)
	go fwder.doForward(ctx)
}

func (fwder *fwderImpl) doForward(ctx context.Context) {
	defer fwder.wg.Done()
	var err error

	dest, err := dests.NewDestination(
		ctx,
		fwder.desttype,
		fwder.conf,
		fwder.store.ACK, fwder.store.NACK, fwder.store.PermError,
		fwder.store.Confined(),
		fwder.logger,
	)
	if err != nil {
		fwder.logger.Error("Error setting up the destination", "error", err)
		fwder.dofatal()
		return
	}

	defer func() {
		// be sure to Close the destination when we are done
		_ = dest.Close()
	}()

	// listen for destination fatal errors
	fwder.wg.Add(1)
	go func() {
		defer fwder.wg.Done()
		select {
		case <-ctx.Done():
			return
		case <-dest.Fatal():
			fwder.dofatal()
		}
	}()

	jsenvs := map[ulid.ULID]*javascript.Environment{}
	done := ctx.Done()
	outputs := fwder.store.Outputs(fwder.desttype)

	var more bool
	var message *model.FullMessage

	for {
		select {
		case <-done:
			return
		case message, more = <-outputs:
			if !more {
				return
			}
			if message != nil {
				err = fwder.fwdMsg(message, jsenvs, dest)
				fwder.store.ReleaseMsg(message)
				if err != nil {
					fwder.logger.Warn("Error forwarding message", "error", err, "uid", ulid.ULID(message.Uid).String())
				}
			}
		}
	}
}

func (fwder *fwderImpl) fwdMsg(m *model.FullMessage, envs map[ulid.ULID]*javascript.Environment, dest dests.Destination) (err error) {
	var errs []error
	var config *conf.FilterSubConfig
	var topic, partitionKey string
	var partitionNumber int32
	var filterResult javascript.FilterResult

	env, ok := envs[m.ConfId]
	if !ok {
		// create the environement for the javascript virtual machine
		config, err = fwder.store.GetSyslogConfig(m.ConfId)
		if err != nil {
			fwder.logger.Warn(
				"could not find the stored configuration for a message",
				"confId", ulid.ULID(m.ConfId).String(),
				"msgId", ulid.ULID(m.Uid).String(),
			)
			fwder.store.PermError(m.Uid, fwder.desttype)
			return err
		}
		envs[m.ConfId] = javascript.NewFilterEnvironment(
			config.FilterFunc,
			config.TopicFunc,
			config.TopicTmpl,
			config.PartitionFunc,
			config.PartitionTmpl,
			config.PartitionNumberFunc,
			fwder.logger,
		)
		env = envs[m.ConfId]
	}

	topic, errs = env.Topic(m.Parsed.Fields)
	for _, err = range errs {
		fwder.logger.Info("Error calculating topic", "error", err, "uid", m.Uid)
	}
	if len(topic) == 0 {
		topic = "default-topic"
	}
	partitionKey, errs = env.PartitionKey(m.Parsed.Fields)
	for _, err := range errs {
		fwder.logger.Info("Error calculating the partition key", "error", err, "uid", m.Uid)
	}
	partitionNumber, errs = env.PartitionNumber(m.Parsed.Fields)
	for _, err := range errs {
		fwder.logger.Info("Error calculating the partition number", "error", err, "uid", m.Uid)
	}

	filterResult, err = env.FilterMessage(&m.Parsed.Fields)

	switch filterResult {
	case javascript.DROPPED:
		fwder.store.ACK(m.Uid, fwder.desttype)
		fwder.messageFilterCounter.WithLabelValues("dropped", m.Parsed.Client, conf.DestinationNames[fwder.desttype]).Inc()
		return
	case javascript.REJECTED:
		fwder.messageFilterCounter.WithLabelValues("rejected", m.Parsed.Client, conf.DestinationNames[fwder.desttype]).Inc()
		fwder.store.NACK(m.Uid, fwder.desttype)
		return
	case javascript.PASS:
		fwder.messageFilterCounter.WithLabelValues("passing", m.Parsed.Client, conf.DestinationNames[fwder.desttype]).Inc()
	default:
		fwder.store.PermError(m.Uid, fwder.desttype)
		fwder.logger.Warn("Error happened processing message", "uid", m.Uid, "error", err)
		fwder.messageFilterCounter.WithLabelValues("unknown", m.Parsed.Client, conf.DestinationNames[fwder.desttype]).Inc()
		return err
	}

	return dest.Send(*m, partitionKey, partitionNumber, topic)
}

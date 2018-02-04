package store

import (
	"context"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/javascript"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/store/dests"
	"github.com/stephane-martin/skewer/sys/binder"
	"github.com/stephane-martin/skewer/utils"
)

type fwderImpl struct {
	logger     log15.Logger
	binder     binder.Client
	wg         *sync.WaitGroup
	once       sync.Once
	store      Store
	conf       conf.BaseConfig
	desttype   conf.DestinationType
	fatalChan  chan struct{}
	outputMsgs []model.OutputMsg
}

func NewForwarder(desttype conf.DestinationType, st Store, bc conf.BaseConfig, logger log15.Logger, bindr binder.Client) (fwder Forwarder) {
	f := fwderImpl{
		logger:     logger.New("class", "forwarder"),
		binder:     bindr,
		store:      st,
		conf:       bc,
		desttype:   desttype,
		fatalChan:  make(chan struct{}),
		outputMsgs: make([]model.OutputMsg, bc.Store.BatchSize),
	}

	f.fatalChan = make(chan struct{})
	f.wg = &sync.WaitGroup{}
	return &f
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
	e := dests.BuildEnv().
		Callbacks(fwder.store.ACK, fwder.store.NACK, fwder.store.PermError).
		Config(fwder.conf).
		Confined(fwder.store.Confined()).
		Logger(fwder.logger).
		Binder(fwder.binder)

	dest, err := dests.NewDestination(ctx, fwder.desttype, e)
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

	jsenvs := map[utils.MyULID]*javascript.Environment{}
	done := ctx.Done()
	outputs := fwder.store.Outputs(fwder.desttype)

	var more bool
	var messages []*model.FullMessage

	for {
		select {
		case <-done:
			return
		case messages, more = <-outputs:
			if !more || messages == nil {
				return
			}
			err = fwder.fwdMsgs(messages, jsenvs, dest)
			if err != nil {
				fwder.logger.Warn("Error forwarding messages", "error", err)
			}
		}
	}
}

func (fwder *fwderImpl) fwdMsgs(msgs []*model.FullMessage, envs map[utils.MyULID]*javascript.Environment, dest dests.Destination) (err error) {
	var errs []error
	var config *conf.FilterSubConfig
	var topic, partitionKey string
	var partitionNumber int32
	var filterResult javascript.FilterResult
	var m *model.FullMessage
	var i int

Loop:
	for _, m = range msgs {
		env, ok := envs[m.ConfId]
		if !ok {
			// create the environment for the javascript virtual machine
			config, err = fwder.store.GetSyslogConfig(m.ConfId)
			if err != nil {
				fwder.logger.Warn(
					"could not find the stored configuration for a message",
					"confId", utils.MyULID(m.ConfId).String(),
					"msgId", utils.MyULID(m.Uid).String(),
				)
				fwder.store.PermError(m.Uid, fwder.desttype)
				continue Loop
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

		_, ok1 := dest.(*dests.KafkaDestination)
		_, ok2 := dest.(*dests.NATSDestination)
		_, ok3 := dest.(*dests.RedisDestination)
		if ok1 || ok2 || ok3 {
			// only calculate proper Topic, PartitionKey and PartitionNumber if we are sending to Kafka or NATS
			topic, errs = env.Topic(m.Fields)
			for _, err = range errs {
				fwder.logger.Info("Error calculating topic", "error", err, "uid", m.Uid)
			}
			if len(topic) == 0 {
				topic = "default-topic"
			}
			partitionKey, errs = env.PartitionKey(m.Fields)
			for _, err := range errs {
				fwder.logger.Info("Error calculating the partition key", "error", err, "uid", m.Uid)
			}
			partitionNumber, errs = env.PartitionNumber(m.Fields)
			for _, err := range errs {
				fwder.logger.Info("Error calculating the partition number", "error", err, "uid", m.Uid)
			}
		} else {
			topic = ""
			partitionKey = ""
			partitionNumber = 0
		}

		filterResult, err = env.FilterMessage(m.Fields)
		if err != nil {
			fwder.logger.Warn("Error happened filtering message", "error", err)
			continue Loop
		}

		switch filterResult {
		case javascript.DROPPED:
			fwder.store.ACK(m.Uid, fwder.desttype)
			messageFilterCounter.WithLabelValues("dropped", m.Fields.GetProperty("skewer", "client"), conf.DestinationNames[fwder.desttype]).Inc()
			continue Loop
		case javascript.REJECTED:
			messageFilterCounter.WithLabelValues("rejected", m.Fields.GetProperty("skewer", "client"), conf.DestinationNames[fwder.desttype]).Inc()
			fwder.store.NACK(m.Uid, fwder.desttype)
			continue Loop
		case javascript.PASS:
			messageFilterCounter.WithLabelValues("passing", m.Fields.GetProperty("skewer", "client"), conf.DestinationNames[fwder.desttype]).Inc()
		default:
			fwder.store.PermError(m.Uid, fwder.desttype)
			fwder.logger.Warn("Error happened processing message", "uid", m.Uid, "error", err)
			messageFilterCounter.WithLabelValues("unknown", m.Fields.GetProperty("skewer", "client"), conf.DestinationNames[fwder.desttype]).Inc()
			continue Loop
		}
		fwder.outputMsgs[i].PartitionKey = partitionKey
		fwder.outputMsgs[i].PartitionNumber = partitionNumber
		fwder.outputMsgs[i].Topic = topic
		fwder.outputMsgs[i].Message = m
		i++
	}
	if i == 0 {
		return nil
	}
	return dest.Send(fwder.outputMsgs[:i], partitionKey, partitionNumber, topic)
}

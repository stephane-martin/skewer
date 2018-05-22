package store

import (
	"context"
	"fmt"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/javascript"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/store/dests"
	"github.com/stephane-martin/skewer/sys/binder"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"go.uber.org/atomic"
)

type Forwarder struct {
	logger     log15.Logger
	binder     binder.Client
	once       sync.Once
	store      *MessageStore
	conf       conf.BaseConfig
	desttype   conf.DestinationType
	outputMsgs []model.OutputMsg
	dest       dests.Destination
}

func NewForwarder(desttype conf.DestinationType, st *MessageStore, bc conf.BaseConfig, logger log15.Logger, bindr binder.Client) *Forwarder {
	f := Forwarder{
		logger:   logger.New("class", "forwarder"),
		binder:   bindr,
		store:    st,
		conf:     bc,
		desttype: desttype,
	}

	return &f
}

func (fwder *Forwarder) CreateDestination(ctx context.Context) (err error) {
	fwder.logger.Debug("Creating destination", "dest", fwder.desttype)
	e := dests.BuildEnv().
		Callbacks(fwder.store.ACK, fwder.store.NACK, fwder.store.PermError).
		Config(fwder.conf).
		Confined(fwder.store.Confined()).
		Logger(fwder.logger).
		Binder(fwder.binder)

	dest, err := dests.NewDestination(ctx, fwder.desttype, e)
	if err != nil {
		return fmt.Errorf("Error setting up the destination: %s", err.Error())
	}
	select {
	case <-ctx.Done():
		return eerrors.New("shutdown")
	default:
		fwder.dest = dest
	}
	return nil
}

func (fwder *Forwarder) Forward(ctx context.Context) (err error) {
	if fwder.dest == nil {
		return eerrors.New("Destination not created for forwarder")
	}

	defer func() {
		// be sure to Close the destination when we are done
		if fwder.dest != nil {
			_ = fwder.dest.Close()
			fwder.dest = nil
		}
	}()

	fwder.outputMsgs = make([]model.OutputMsg, fwder.conf.Store.BatchSize)
	jsenvs := map[utils.MyULID]*javascript.Environment{}
	outputs := fwder.store.Outputs(fwder.desttype)

	var more bool
	var messages []*model.FullMessage
	var stopping atomic.Bool
	shutdown := false
	var rerr error

	go func() {
		select {
		case <-ctx.Done():
			shutdown = true
			stopping.Store(true)
		case err := <-fwder.dest.Fatal():
			rerr = err
			stopping.Store(true)
		}
	}()

	for {
		select {

		case <-ctx.Done():
			return nil
		case err := <-fwder.dest.Fatal():
			return err
		case messages, more = <-outputs:
			if !more || messages == nil {
				return nil
			}
			if stopping.Load() {
				fwder.dest.NACKAllSlice(messages)
				if shutdown {
					return nil
				}
				return rerr
			}
			errs := fwder.fwdMsgs(ctx, messages, jsenvs, fwder.dest)
			if errs != nil {
				fwder.logger.Warn("Errors forwarding messages", "errors", errs)
			}
		}
	}
}

func (fwder *Forwarder) fwdMsgs(ctx context.Context, msgs []*model.FullMessage, envs map[utils.MyULID]*javascript.Environment, dest dests.Destination) (err eerrors.ErrorSlice) {

	i := int(0)

Loop:
	for _, m := range msgs {
		if m == nil || m.Fields == nil {
			continue Loop
		}
		env, ok := envs[m.ConfId]
		if !ok {
			// create the environment for the javascript virtual machine
			config, e := fwder.store.GetSyslogConfig(m.ConfId)
			if e != nil {
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

		topic := ""
		partitionKey := ""
		partitionNumber := int32(0)
		var joinedErr error

		_, ok1 := dest.(*dests.KafkaDestination)
		_, ok2 := dest.(*dests.NATSDestination)
		_, ok3 := dest.(*dests.RedisDestination)

		if ok1 || ok2 || ok3 {
			// only calculate proper Topic, PartitionKey and PartitionNumber if we are sending to Kafka or NATS
			topic, joinedErr = env.Topic(m.Fields)
			if joinedErr != nil {
				fwder.logger.Info("Error calculating topic", "error", joinedErr.Error(), "uid", m.Uid)
			}
			if len(topic) == 0 {
				topic = "default-topic"
			}
			partitionKey, joinedErr = env.PartitionKey(m.Fields)
			if joinedErr != nil {
				fwder.logger.Info("Error calculating the partition key", "error", err, "uid", m.Uid)
			}
			partitionNumber, joinedErr = env.PartitionNumber(m.Fields)
			if joinedErr != nil {
				fwder.logger.Info("Error calculating the partition number", "error", err, "uid", m.Uid)
			}
		}

		filterResult, e := env.FilterMessage(m.Fields)
		if e != nil {
			fwder.logger.Warn("Error happened filtering message", "error", e)
			continue Loop
		}

		switch filterResult {
		case javascript.DROPPED:
			fwder.store.ACK(m.Uid, fwder.desttype)
			countFiltered(fwder.desttype, "dropped", m.Fields.GetProperty("skewer", "client"))
			continue Loop
		case javascript.REJECTED:
			fwder.store.NACK(m.Uid, fwder.desttype)
			countFiltered(fwder.desttype, "rejected", m.Fields.GetProperty("skewer", "client"))
			continue Loop
		case javascript.PASS:
			countFiltered(fwder.desttype, "passing", m.Fields.GetProperty("skewer", "client"))
		default:
			fwder.store.PermError(m.Uid, fwder.desttype)
			countFiltered(fwder.desttype, "unknown", m.Fields.GetProperty("skewer", "client"))
			fwder.logger.Warn("Error happened processing message", "uid", m.Uid, "error", err)
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
	return dest.Send(ctx, fwder.outputMsgs[:i])
}

package dests

import (
	"context"
	"strings"

	nats "github.com/nats-io/go-nats"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/encoders"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"github.com/valyala/bytebufferpool"
)

type NATSDestination struct {
	*baseDestination
	conn *nats.Conn
}

func NewNATSDestination(ctx context.Context, e *Env) (Destination, error) {
	config := e.config.NATSDest
	d := &NATSDestination{
		baseDestination: newBaseDestination(conf.NATS, "nats", e),
	}
	err := d.setFormat(config.Format)
	if err != nil {
		return nil, err
	}
	opts := nats.Options{
		Name:             config.Name,
		MaxReconnect:     config.MaxReconnect,
		ReconnectWait:    config.ReconnectWait,
		Timeout:          config.ConnTimeout,
		PingInterval:     config.PingInterval,
		MaxPingsOut:      config.MaxPingsOut,
		ReconnectBufSize: config.ReconnectBufSize,
		NoRandomize:      config.NoRandomize,
		FlusherTimeout:   config.FlusherTimeout,
		ClosedCB:         d.closeHandler,
		DisconnectedCB:   d.disconnectHandler,
		ReconnectedCB:    d.reconnectHandler,
		AllowReconnect:   config.AllowReconnect,
		SubChanLen:       nats.DefaultMaxChanLen,
		Servers:          config.NServers,
	}
	username := strings.TrimSpace(config.Username)
	password := strings.TrimSpace(config.Password)
	if len(username) > 0 && len(password) > 0 {
		opts.User = username
		opts.Password = password
	}
	if config.TLSEnabled {
		opts.Secure = true
		tlsconfig, err := utils.NewTLSConfig("", config.CAFile, config.CAPath, config.CertFile, config.KeyFile, config.Insecure, e.confined)
		if err != nil {
			return nil, err
		}
		opts.TLSConfig = tlsconfig
	}

	conn, err := opts.Connect()
	if err != nil {
		return nil, err
	}
	d.conn = conn
	return d, nil
}

func (d *NATSDestination) closeHandler(conn *nats.Conn) {
	d.logger.Debug("NATS client has been closed")
	d.dofatal()
}

func (d *NATSDestination) disconnectHandler(conn *nats.Conn) {
	d.logger.Warn("NATS client has been disconnected")
}

func (d *NATSDestination) reconnectHandler(conn *nats.Conn) {
	d.logger.Info("NATS client has reconnected")
}

func (d *NATSDestination) Close() error {
	d.conn.Close()
	return nil
}

func (d *NATSDestination) sendOne(msg *model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)
	err = d.encoder(msg, buf)
	if err != nil {
		return err
	}
	// we use buf.String() to get a copy of buf, so that we can release buf afterwards
	return d.conn.Publish(topic, []byte(buf.String()))
}

func (d *NATSDestination) Send(ctx context.Context, msgs []model.OutputMsg, partitionKey string, partitionNumber int32, topic string) (err error) {
	var e error
	var msg *model.FullMessage
	var uid utils.MyULID
	for len(msgs) > 0 {
		msg = msgs[0].Message
		uid = msg.Uid
		e = d.sendOne(msg, msgs[0].PartitionKey, msgs[0].PartitionNumber, msgs[0].Topic)
		model.FullFree(msg)
		msgs = msgs[1:]
		if e != nil {
			if encoders.IsEncodingError(e) {
				d.PermError(uid)
			} else {
				d.NACK(uid)
				d.NACKRemaining(msgs)
				d.dofatal()
				return e
			}
			if err == nil {
				err = e
			}
		} else {
			d.ACK(msg.Uid)
		}
	}
	return err
}

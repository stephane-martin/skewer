package dests

import (
	"bytes"
	"context"
	"strings"

	nats "github.com/nats-io/go-nats"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
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
	defer model.FullFree(msg)

	buf := bytes.NewBuffer(nil)
	err = d.encoder(msg, buf)
	if err != nil {
		d.permerr(msg.Uid)
		return err
	}
	err = d.conn.Publish(topic, buf.Bytes())
	if err != nil {
		d.nack(msg.Uid)
		d.dofatal()
		return err
	}
	d.ack(msg.Uid)
	return nil
}

func (d *NATSDestination) Send(msgs []model.OutputMsg, partitionKey string, partitionNumber int32, topic string) (err error) {
	var i int
	var e error
	for i = range msgs {
		e = d.sendOne(msgs[i].Message, msgs[i].PartitionKey, msgs[i].PartitionNumber, msgs[i].Topic)
		if e != nil {
			err = e
		}
	}
	return err
}

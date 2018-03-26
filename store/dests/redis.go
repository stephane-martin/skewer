package dests

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/encoders"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
)

type RedisDestination struct {
	*baseDestination
	client *redis.Client
}

func NewRedisDestination(ctx context.Context, e *Env) (Destination, error) {
	config := e.config.RedisDest
	d := &RedisDestination{
		baseDestination: newBaseDestination(conf.Elasticsearch, "elasticsearch", e),
	}
	err := d.setFormat(config.Format)
	if err != nil {
		return nil, err
	}

	opts := &redis.Options{
		Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
		Network:      "tcp",
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		DB:           config.Database,
	}
	if len(config.Password) > 0 {
		opts.Password = config.Password
	}
	if config.TLSEnabled {
		tlsConf, err := utils.NewTLSConfig("", config.CAFile, config.CAPath, config.CertFile, config.KeyFile, config.Insecure, e.confined)
		if err != nil {
			return nil, err
		}
		opts.TLSConfig = tlsConf
	}
	client := redis.NewClient(opts)
	_, err = client.Ping().Result()
	if err != nil {
		return nil, err
	}
	d.client = client

	if config.Rebind > 0 {
		go func() {
			select {
			case <-ctx.Done():
				// the store service asked for stop
			case <-time.After(config.Rebind):
				e.logger.Info("HTTP destination rebind period has expired", "rebind", config.Rebind.String())
				d.dofatal()
			}
		}()
	}

	return d, nil
}

func (d *RedisDestination) Close() error {
	return d.client.Close()
}

func (d *RedisDestination) sendOne(ctx context.Context, msg *model.FullMessage, topic, pKey string, pNumber int32) (err error) {
	var buf string
	buf, err = encoders.ChainEncode(d.encoder, msg)
	if err != nil {
		return err
	}
	_, err = d.client.RPush(topic, []byte(buf)).Result()
	return err
}

func (d *RedisDestination) Send(ctx context.Context, msgs []model.OutputMsg, partitionKey string, partitionNumber int32, topic string) (err error) {
	return d.ForEachWithTopic(ctx, d.sendOne, d.ACK, msgs)
}

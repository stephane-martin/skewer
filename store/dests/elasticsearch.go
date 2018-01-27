package dests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/olivere/elastic"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/model/encoders"
	"github.com/stephane-martin/skewer/utils"
)

type ElasticDestination struct {
	*baseDestination
	elasticlient *elastic.Client
	processor    *elastic.BulkProcessor
	indexNameTpl *template.Template
	messagesType string
}

func NewElasticDestination(ctx context.Context, e *Env) (Destination, error) {
	config := e.config.ElasticDest
	if len(config.URLs) == 0 {
		config.URLs = []string{"http://127.0.0.1:9200"}
	}
	d := &ElasticDestination{
		baseDestination: newBaseDestination(conf.Elasticsearch, "elasticsearch", e),
		messagesType:    config.MessagesType,
	}
	var err error
	d.indexNameTpl, err = template.New("index").Parse(config.IndexNameTpl)
	if err != nil {
		return nil, err
	}
	err = d.setFormat(config.Format)
	if err != nil {
		return nil, err
	}

	config.ProxyURL = strings.TrimSpace(config.ProxyURL)

	if strings.HasPrefix(strings.ToLower(config.URLs[0]), "https") {
		config.TLSEnabled = true
	}
	dialer := &net.Dialer{
		Timeout: config.ConnTimeout,
	}
	if config.ConnKeepAlive {
		dialer.KeepAlive = config.ConnKeepAlivePeriod
	}

	transport := &http.Transport{
		MaxIdleConnsPerHost:   http.DefaultMaxIdleConnsPerHost,
		IdleConnTimeout:       90 * time.Second,
		Proxy:                 nil,
		MaxIdleConns:          100,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		DialContext:           dialer.DialContext,
	}

	if config.TLSEnabled {
		tlsconfig, err := utils.NewTLSConfig(
			"",
			config.CAFile,
			config.CAPath,
			config.CertFile,
			config.KeyFile,
			config.Insecure,
			e.confined,
		)
		if err != nil {
			return nil, err
		}
		transport.TLSClientConfig = tlsconfig
	}

	if len(config.ProxyURL) > 0 {
		url, err := url.Parse(config.ProxyURL)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(url)
	}

	httpClient := &http.Client{
		Transport: transport,
		Jar:       nil,
	}

	elasticOpts := []elastic.ClientOptionFunc{}
	elasticOpts = append(elasticOpts, elastic.SetURL(config.URLs...))
	elasticOpts = append(elasticOpts, elastic.SetHttpClient(httpClient))
	elasticOpts = append(elasticOpts, elastic.SetErrorLog(&myLogger{logger: d.logger}))
	elasticOpts = append(elasticOpts, elastic.SetHealthcheck(config.HealthCheck))
	elasticOpts = append(elasticOpts, elastic.SetSniff(config.Sniffing))
	if config.HealthCheck {
		elasticOpts = append(elasticOpts, elastic.SetHealthcheckTimeout(config.HealthCheckTimeout))
		elasticOpts = append(elasticOpts, elastic.SetHealthcheckTimeoutStartup(config.HealthCheckTimeoutStartup))
		elasticOpts = append(elasticOpts, elastic.SetHealthcheckInterval(config.HealthCheckInterval))
	}

	if config.TLSEnabled {
		elasticOpts = append(elasticOpts, elastic.SetScheme("https"))
	}
	if len(config.Username) > 0 && len(config.Password) > 0 {
		elasticOpts = append(elasticOpts, elastic.SetBasicAuth(config.Username, config.Password))
	}

	d.elasticlient, err = elastic.NewClient(elasticOpts...)
	if err != nil {
		return nil, err
	}

	processor := d.elasticlient.BulkProcessor().
		Name("SkewerWorker").
		Workers(http.DefaultMaxIdleConnsPerHost).
		BulkActions(config.BatchSize).
		Stats(true).
		FlushInterval(config.FlushPeriod).
		After(d.after).
		Backoff(elastic.StopBackoff{})

	d.processor, err = processor.Do(context.Background())
	if err != nil {
		return nil, err
	}

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

func (d *ElasticDestination) after(executionId int64, requests []elastic.BulkableRequest, response *elastic.BulkResponse, err error) {
	if response == nil {
		d.dofatal()
		return
	}
	successes := response.Succeeded()
	failures := response.Failed()
	var item *elastic.BulkResponseItem
	var uid utils.MyULID
	var e error
	for _, item = range successes {
		uid, e = utils.Parse(item.Id)
		if e != nil {
			continue
		}
		d.ack(uid)
	}
	if len(failures) == 0 {
		return
	}

	for _, item = range failures {
		uid, e = utils.Parse(item.Id)
		if e != nil {
			continue
		}
		d.nack(uid)
		if item.Error != nil {
			d.logger.Warn("Elasticsearch index error", "type", item.Error.Type, "reason", item.Error.Reason, "index", item.Error.Index)
		}
	}
	d.dofatal()
}

func (d *ElasticDestination) Close() error {
	return d.processor.Close()
}

func (d *ElasticDestination) Send(msg *model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	indexBuf := bytes.NewBuffer(nil)
	err = d.indexNameTpl.Execute(indexBuf, msg.Fields)
	if err != nil {
		d.permerr(msg.Uid)
		return err
	}
	var buf json.RawMessage
	buf, err = encoders.ChainEncode(d.encoder, msg)
	if err != nil {
		d.permerr(msg.Uid)
		return err
	}
	d.processor.Add(
		elastic.NewBulkIndexRequest().Index(indexBuf.String()).Type(d.messagesType).Id(msg.Uid.String()).Doc(buf),
	)
	return nil
}

type myLogger struct {
	logger log15.Logger
}

func (l *myLogger) Printf(format string, v ...interface{}) {
	l.logger.Warn(fmt.Sprintf(format, v...))
}

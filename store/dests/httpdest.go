package dests

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/queue/message"
)

type HTTPDestination struct {
	logger      log15.Logger
	fatal       chan struct{}
	ack         storeCallback
	nack        storeCallback
	permerr     storeCallback
	clt         *http.Client
	once        sync.Once
	username    string
	password    string
	useragent   string
	url         *template.Template
	method      string
	contentType string
	sendQueue   *message.Ring
	encoder     model.Encoder
	wg          sync.WaitGroup
}

func NewHTTPDestination(ctx context.Context, cfnd bool, bc conf.BaseConfig, ack, nack, pe storeCallback, l log15.Logger) (d *HTTPDestination, err error) {
	conf := bc.HTTPDest
	d = &HTTPDestination{
		logger:    l,
		fatal:     make(chan struct{}),
		ack:       ack,
		nack:      nack,
		permerr:   pe,
		useragent: conf.UserAgent,
		method:    conf.Method,
	}

	encoder, err := model.NewEncoder(conf.Format)
	if err != nil {
		return nil, err
	}
	d.encoder = encoder

	conf.ContentType = strings.TrimSpace(strings.ToLower(conf.ContentType))
	d.contentType = conf.ContentType
	if conf.ContentType == "auto" {
		switch conf.Format {
		case "json", "fulljson", "gelf":
			d.contentType = "application/json"
		default:
			d.contentType = mime.FormatMediaType("text/plain", map[string]string{"charset": "utf-8"})
		}
	}

	if conf.BasicAuth {
		d.username = conf.Username
		d.password = conf.Password
	}

	conf.URL = strings.TrimSpace(conf.URL)
	conf.ProxyURL = strings.TrimSpace(conf.ProxyURL)

	zurl, err := url.Parse(conf.URL)
	if err != nil {
		return nil, err
	}
	host := zurl.Host

	tmpl, err := template.New("url").Parse(conf.URL)
	if err != nil {
		return nil, err
	}
	d.url = tmpl

	if strings.HasPrefix(strings.ToLower(conf.URL), "https") {
		conf.TLSEnabled = true
	}
	dialer := &net.Dialer{
		Timeout: conf.ConnTimeout,
	}
	if conf.ConnKeepAlive {
		dialer.KeepAlive = conf.ConnKeepAlivePeriod
	}

	transport := &http.Transport{
		MaxIdleConnsPerHost:   conf.MaxIdleConnsPerHost,
		IdleConnTimeout:       conf.IdleConnTimeout,
		Proxy:                 nil,
		MaxIdleConns:          100,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		DialContext:           dialer.DialContext,
	}

	if conf.TLSEnabled {
		tlsconfig, err := utils.NewTLSConfig(
			host,
			conf.CAFile,
			conf.CAPath,
			conf.CertFile,
			conf.KeyFile,
			conf.Insecure,
			cfnd,
		)
		if err != nil {
			return nil, err
		}
		transport.TLSClientConfig = tlsconfig
	}

	if len(conf.ProxyURL) > 0 {
		url, err := url.Parse(conf.ProxyURL)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(url)
	}

	d.clt = &http.Client{
		Transport: transport,
		Jar:       nil,
	}
	d.sendQueue = message.NewRing(4 * uint64(conf.MaxIdleConnsPerHost))

	if conf.Rebind > 0 {
		go func() {
			select {
			case <-ctx.Done():
				// the store service asked for stop
			case <-time.After(conf.Rebind):
				l.Info("HTTP destination rebind period has expired", "rebind", conf.Rebind.String())
				d.dofatal()
			}
		}()
	}

	for i := 0; i < conf.MaxIdleConnsPerHost; i++ {
		d.wg.Add(1)
		go d.dosend(ctx)
	}

	return d, nil
}

func (d *HTTPDestination) dofatal() {
	d.once.Do(func() { close(d.fatal) })
}

func (d *HTTPDestination) Close() error {
	d.sendQueue.Dispose()
	d.wg.Wait()
	return nil
}

func (d *HTTPDestination) Fatal() chan struct{} {
	return d.fatal
}

func (d *HTTPDestination) dosend(ctx context.Context) {
	defer d.wg.Done()
	for {
		msg, err := d.sendQueue.Get()
		if err != nil {
			return
		}
		urlbuf := bytes.NewBuffer(nil)
		err = d.url.Execute(urlbuf, msg.Parsed)
		if err != nil {
			d.permerr(msg.Uid, conf.HTTP)
			ackCounter.WithLabelValues("http", "permerr", "").Inc()
			d.logger.Warn("Error calculating target URL from template", "error", err)
			continue
		}
		body := bytes.NewBuffer(nil)
		err = d.encoder.Enc(msg, body)
		if err != nil {
			d.permerr(msg.Uid, conf.HTTP)
			ackCounter.WithLabelValues("http", "permerr", "").Inc()
			d.logger.Warn("Error encoding message", "error", err)
			continue
		}
		req, err := http.NewRequest(d.method, urlbuf.String(), body)
		if err != nil {
			d.permerr(msg.Uid, conf.HTTP)
			ackCounter.WithLabelValues("http", "permerr", "").Inc()
			d.logger.Warn("Error preparing HTTP request", "error", err)
			continue
		}
		req.Header.Set("Content-Type", d.contentType)
		if len(d.useragent) > 0 {
			req.Header.Set("User-Agent", d.useragent)
		}
		if len(d.username) > 0 && len(d.password) > 0 {
			req.SetBasicAuth(d.username, d.password)
		}
		req = req.WithContext(ctx)
		resp, err := d.clt.Do(req)
		if err != nil {
			// server down ?
			d.nack(msg.Uid, conf.HTTP)
			ackCounter.WithLabelValues("http", "nack", "").Inc()
			fatalCounter.WithLabelValues("http").Inc()
			d.logger.Warn("Error sending HTTP request", "error", err)
			d.dofatal()
			return
		}
		// not interested in response body
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()

		httpStatusCounter.WithLabelValues(req.Host, strconv.FormatInt(int64(resp.StatusCode), 10)).Inc()

		if 200 <= resp.StatusCode && resp.StatusCode < 300 {
			d.ack(msg.Uid, conf.HTTP)
			ackCounter.WithLabelValues("http", "ack", "").Inc()
			continue
		}
		if 400 <= resp.StatusCode && resp.StatusCode < 500 {
			// client-side error ??!
			d.nack(msg.Uid, conf.HTTP)
			ackCounter.WithLabelValues("http", "nack", "").Inc()
			fatalCounter.WithLabelValues("http").Inc()
			d.logger.Warn("Client side error sending HTTP request", "code", resp.StatusCode, "status", resp.Status)
			d.dofatal()
			return
		}
		if 500 <= resp.StatusCode && resp.StatusCode < 600 {
			// server side error
			d.nack(msg.Uid, conf.HTTP)
			ackCounter.WithLabelValues("http", "nack", "").Inc()
			fatalCounter.WithLabelValues("http").Inc()
			d.logger.Warn("Server side error sending HTTP request", "code", resp.StatusCode, "status", resp.Status)
			d.dofatal()
			return
		}
		d.nack(msg.Uid, conf.HTTP)
		ackCounter.WithLabelValues("http", "nack", "").Inc()
		fatalCounter.WithLabelValues("http").Inc()
		d.logger.Warn("Unexpected status code sending HTTP request", "code", resp.StatusCode, "status", resp.Status)
		d.dofatal()
		return
	}
}

func (d *HTTPDestination) Send(msg model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	err = d.sendQueue.Put(&msg)
	if err != nil {
		// the client send queue has been disposed
		ackCounter.WithLabelValues("http", "nack", "").Inc()
		fatalCounter.WithLabelValues("http").Inc()
		d.nack(msg.Uid, conf.HTTP)
		d.dofatal()
	}
	return err
}

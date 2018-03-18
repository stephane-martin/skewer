package dests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/encoders"
	"github.com/stephane-martin/skewer/encoders/baseenc"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/queue/message"
)

type HTTPServerDestination struct {
	*baseDestination
	contentType string
	sendQueue   *message.Ring
	server      *http.Server
	wg          sync.WaitGroup
	nMessages   int
	lineFraming bool
	delimiter   uint8
}

func NewHTTPServerDestination(ctx context.Context, e *Env) (Destination, error) {
	config := e.config.HTTPServerDest
	var err error
	var listener net.Listener

	d := &HTTPServerDestination{
		baseDestination: newBaseDestination(conf.HTTPServer, "httpserver", e),
		lineFraming:     config.LineFraming,
		delimiter:       config.FrameDelimiter,
	}

	d.nMessages = int(config.NMessages)
	if config.NMessages <= 0 {
		d.nMessages = 8 * 1024
	}

	if len(config.Format) > 0 {
		// determine content-type from the fixed output format
		err = d.setFormat(config.Format)
		if err != nil {
			return nil, err
		}
		if d.nMessages == 1 {
			d.contentType = encoders.MimeTypes[d.format]
			if d.contentType == "" {
				// should not happen ??
				return nil, fmt.Errorf("Unknown mimetype for that format: '%d'", d.format)
			}
		} else {
			switch d.format {
			case baseenc.JSON, baseenc.GELF:
				if config.LineFraming {
					if config.FrameDelimiter == 10 {
						// Newline delimited JSON
						d.contentType = encoders.NDJsonMimetype
					} else {
						// custom delimiter => text/plain
						d.contentType = encoders.PlainMimetype
					}
				} else {
					// octet counting frames => text/plain
					d.contentType = encoders.PlainMimetype
				}
			case baseenc.Protobuf:
				// protobuf is not natively self delimited, so we use octet counting framing
				d.contentType = encoders.OctetStreamMimetype
				d.lineFraming = false
			case baseenc.RFC5424, baseenc.RFC3164, baseenc.File:
				d.contentType = encoders.PlainMimetype
			default:
				return nil, fmt.Errorf("Unknown format: '%d'", d.format)
			}
		}
	}
	hostport := net.JoinHostPort(config.BindAddr, strconv.FormatInt(int64(config.Port), 10))
	if config.DisableConnKeepAlive {
		listener, err = d.binder.Listen("tcp", hostport)
	} else {
		listener, err = d.binder.ListenKeepAlive("tcp", hostport, config.ConnKeepAlivePeriod)
	}
	if err != nil {
		return nil, err
	}
	d.server = &http.Server{
		Handler:           d,
		ReadTimeout:       config.ReadTimeout,
		ReadHeaderTimeout: config.ReadTimeout,
		WriteTimeout:      config.WriteTimeout,
		IdleTimeout:       config.IdleTimeout,
		MaxHeaderBytes:    config.MaxHeaderBytes,
		ErrorLog:          log.New(d, "", 0),
	}
	d.server.SetKeepAlivesEnabled(!config.DisableHTTPKeepAlive)
	if config.TLSEnabled {
		tlsConf, err := utils.NewTLSConfig("", config.CAFile, config.CAPath, config.CertFile, config.KeyFile, false, d.confined)
		if err != nil {
			return nil, err
		}
		tlsConf.ClientAuth = config.GetClientAuthType()
		d.server.TLSConfig = tlsConf
	}
	d.sendQueue = message.NewRing(uint64(d.nMessages))
	d.wg.Add(1)
	go func() {
		<-ctx.Done()
		d.sendQueue.Dispose()
		d.wg.Done()
	}()
	d.wg.Add(1)
	go d.serve(listener)

	return d, nil
}

func (d *HTTPServerDestination) Write(p []byte) (n int, err error) {
	// trick the http server to write logs to the destination logger
	d.logger.Debug(string(bytes.TrimSpace(p)))
	return len(p), nil
}

func (d *HTTPServerDestination) getContentType(r *http.Request) (ctype string) {
	if d.contentType != "" {
		return d.contentType
	}
	ctype = utils.NegotiateContentType(r, encoders.AcceptedMimeTypes, encoders.NDJsonMimetype)
	if ctype == "text/plain" {
		ctype = encoders.PlainMimetype
	}
	return ctype
}

func (d *HTTPServerDestination) getEncoder(ctype string) encoders.Encoder {
	if d.encoder != nil {
		return d.encoder
	}
	return encoders.RMimeTypes[ctype]
}

func (d *HTTPServerDestination) serve(listener net.Listener) (err error) {
	defer func() {
		if err != nil {
			d.logger.Info("HTTP server stopped", "error", err)
		}
		listener.Close()
		d.dofatal()
		d.wg.Done()
	}()
	if d.server.TLSConfig == nil {
		return d.server.Serve(listener)
	}
	return d.server.ServeTLS(listener, "", "")
}

func (d *HTTPServerDestination) nackall(messages []*model.FullMessage) {
	var message *model.FullMessage
	for _, message = range messages {
		d.nack(message.Uid)
		model.FullFree(message)
	}
}

func (d *HTTPServerDestination) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	messages := make([]*model.FullMessage, 0, d.nMessages)
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	lineFraming := d.lineFraming
	nMessages := d.nMessages
	delimiter := d.delimiter

	contentType := d.getContentType(r)
	if contentType == encoders.JsonMimetype || contentType == encoders.ProtobufMimetype {
		nMessages = 1
		lineFraming = true
	} else if contentType == encoders.NDJsonMimetype {
		delimiter = 10
		lineFraming = true
	} else if contentType == encoders.OctetStreamMimetype {
		lineFraming = false
	}

	encodr := d.getEncoder(contentType)
	if encodr == nil {
		d.logger.Error("getEncoder returned nil", "content-type", contentType)
		w.WriteHeader(http.StatusInternalServerError)
		d.dofatal()
		return
	}

	// wait for first message
	message, err := d.sendQueue.Get()
	if err != nil {
		// sendQueue has been closed
		w.WriteHeader(http.StatusServiceUnavailable)
		d.dofatal()
		return
	}
	messages = append(messages, message)

	// gather additional messages
Loop:
	for len(messages) < nMessages {
		message, err = d.sendQueue.Poll(10 * time.Millisecond)
		if err == utils.ErrTimeout {
			break Loop
		} else if err == utils.ErrDisposed || message == nil {
			// the sendQueue has been closed, definitely no more messages
			defer d.dofatal()
			break Loop
		} else {
			messages = append(messages, message)
		}
	}

	select {
	case <-r.Context().Done():
		// client is gone
		d.nackall(messages)
		return
	default:
	}

	// send the messages to the client
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)

	var buf []byte
	var i int
	last := len(messages) - 1
	permerrors := make(map[utils.MyULID]bool)
	ok := true

	for i, message = range messages {
		if lineFraming {
			if i == last {
				buf, err = encoders.ChainEncode(encodr, message)
			} else {
				buf, err = encoders.ChainEncode(encodr, message, []byte{delimiter})
			}
		} else {
			buf, err = encoders.TcpOctetEncode(encodr, message)
		}
		if err != nil {
			// error encoding one message
			permerrors[message.Uid] = true
		} else {
			_, err = w.Write(buf)
			if err != nil {
				// client is gone
				ok = false
				break
			}
		}
	}

	for _, message = range messages {
		if permerrors[message.Uid] {
			d.permerr(message.Uid)
		} else if ok {
			d.ack(message.Uid)
		} else {
			d.nack(message.Uid)
		}
		model.FullFree(message)
	}
}

func (d *HTTPServerDestination) Close() (err error) {
	d.sendQueue.Dispose()
	err = d.server.Close()
	d.wg.Wait()
	// nack remaining messages
	var message *model.FullMessage
	var e error
	for {
		message, e = d.sendQueue.Get()
		if e != nil || message == nil {
			break
		}
		d.nack(message.Uid)
		model.FullFree(message)
	}
	return err
}

func (d *HTTPServerDestination) Send(msgs []model.OutputMsg, partitionKey string, partitionNumber int32, topic string) (err error) {
	var i int
	for len(msgs) > 0 {
		err = d.sendQueue.Put(msgs[0].Message)
		if err != nil {
			for i = range msgs {
				d.nack(msgs[i].Message.Uid)
				model.FullFree(msgs[i].Message)
			}
			return err
		}
		msgs = msgs[1:]
	}
	return nil
}

package dests

import (
	"context"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/model/encoders"
)

type HTTPServerDestination struct {
	*baseDestination
	contentType string
	sendQueue   chan *model.FullMessage
	server      *http.Server
	wg          sync.WaitGroup
	nMessages   int
	lineFraming bool
	delimiter   uint8
}

func NewHTTPServerDestination(ctx context.Context, e *Env) (Destination, error) {
	config := e.config.HTTPServerDest

	d := &HTTPServerDestination{
		baseDestination: newBaseDestination(conf.HTTPServer, "httpserver", e),
		lineFraming:     config.LineFraming,
		delimiter:       config.FrameDelimiter,
	}

	if config.NMessages <= 0 {
		d.nMessages = 8 * 1024
	} else {
		d.nMessages = int(config.NMessages)
	}

	err := d.setFormat(config.Format)
	if err != nil {
		return nil, err
	}

	// set appropriate content type header
	// TODO: follow what client says
	config.ContentType = strings.TrimSpace(strings.ToLower(config.ContentType))
	d.contentType = config.ContentType
	if config.ContentType == "auto" || config.ContentType == "" {
		if d.nMessages == 1 {
			d.contentType = encoders.MimeTypes[d.format]
			if d.contentType == "" {
				// should not happen ??
				// TODO: log
				d.contentType = "application/octet-stream"
			}
		} else {
			switch d.format {
			case encoders.FullJSON, encoders.JSON, encoders.GELF:
				if config.LineFraming {
					if config.FrameDelimiter == 10 {
						// Newline delimited JSON
						d.contentType = "application/x-ndjson"
					} else {
						// custom delimiter => text/plain
						d.contentType = encoders.MimeTypes[encoders.RFC5424]
					}
				} else {
					// octet counting frames => text/plain
					d.contentType = encoders.MimeTypes[encoders.RFC5424]
				}
			case encoders.Protobuf:
				// protobuf is not natively self delimited
				d.contentType = "application/octet-stream"
			default:
				// text/plain and charset utf-8
				d.contentType = encoders.MimeTypes[encoders.RFC5424]
			}
		}
	}

	d.sendQueue = make(chan *model.FullMessage, d.nMessages)

	hostport := net.JoinHostPort(config.BindAddr, strconv.FormatInt(int64(config.Port), 10))
	d.server = &http.Server{
		Addr:    hostport,
		Handler: d,
	}
	d.wg.Add(1)
	go d.serve()

	return d, nil
}

func (d *HTTPServerDestination) serve() (err error) {
	defer func() {
		d.dofatal()
		d.wg.Done()
	}()
	// TODO: use binder
	err = d.server.ListenAndServe()
	return err
}

func (d *HTTPServerDestination) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//fmt.Fprintln(os.Stderr, "serveHTTP")
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// wait for first message
	var firstMsg *model.FullMessage
	var ok bool
	select {
	case firstMsg, ok = <-d.sendQueue:
		if !ok || firstMsg == nil {
			// sendQueue has been closed
			w.WriteHeader(http.StatusServiceUnavailable)
			d.dofatal()
			return
		}
	case <-r.Context().Done():
		// client is gone
		return
	}
	messages := make([]*model.FullMessage, 0, d.nMessages)
	messages = append(messages, firstMsg)

	// gather additional messages
	var message *model.FullMessage
	ok = true
Loop:
	for len(messages) < d.nMessages {
		select {
		case <-r.Context().Done():
			// client is gone
			for _, message = range messages {
				d.nack(message.Uid)
			}
			return
		case message, ok = <-d.sendQueue:
			if !ok || message == nil {
				// the sendQueue has been closed, definitely no more messages
				break Loop
			}
			messages = append(messages, message)
		case <-time.After(10 * time.Millisecond):
			// no more messages are available in sendQueue
			break Loop
		}
	}

	select {
	case <-r.Context().Done():
		// client is gone
		for _, message = range messages {
			d.nack(message.Uid)
		}
		return
	default:
	}

	// send the messages to the client
	w.Header().Set("Content-Type", d.contentType)
	w.WriteHeader(http.StatusOK)

	var buf []byte
	var err error
	for _, message := range messages {
		if d.lineFraming {
			buf, err = encoders.ChainEncode(d.encoder, message, []byte{d.delimiter})
		} else {
			buf, err = encoders.TcpOctetEncode(d.encoder, message)
		}
		if err != nil {
			d.permerr(message.Uid)
			continue
		}
		_, err = w.Write(buf)
		if err != nil {
			d.nack(message.Uid)
			break
		}
		d.ack(message.Uid)
	}

	if !ok {
		// the sendQueue has been closed, no more messages
		d.dofatal()
	}
}

func (d *HTTPServerDestination) Close() (err error) {
	// Send will not be called again, we can close the sendQueue
	close(d.sendQueue)
	err = d.server.Close()
	d.wg.Wait()
	// nack remaining messages
	for message := range d.sendQueue {
		d.nack(message.Uid)
	}
	return err
}

func (d *HTTPServerDestination) Send(msg *model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	d.sendQueue <- msg
	return nil
}

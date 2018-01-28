package dests

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/model/encoders"
)

const writeWait = 10 * time.Second
const pongWait = 60 * time.Second
const pingPeriod = (pongWait * 9) / 10

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type WebsocketServerDestination struct {
	*baseDestination
	sendQueue   chan *model.FullMessage
	server      *http.Server
	messageType int
	wg          sync.WaitGroup
	mu          sync.Mutex
	connections map[*websocket.Conn]bool
	stopchan    <-chan struct{}
}

func NewWebsocketServerDestination(ctx context.Context, e *Env) (Destination, error) {
	config := e.config.WebsocketServerDest

	d := &WebsocketServerDestination{
		baseDestination: newBaseDestination(conf.WebsocketServer, "websocketserver", e),
		connections:     make(map[*websocket.Conn]bool),
		stopchan:        ctx.Done(),
	}
	err := d.setFormat(config.Format)
	if err != nil {
		return nil, err
	}

	switch d.format {
	case encoders.Protobuf:
		d.messageType = websocket.BinaryMessage
	default:
		d.messageType = websocket.TextMessage
	}

	d.sendQueue = make(chan *model.FullMessage, 1024)

	hostport := net.JoinHostPort(config.BindAddr, strconv.FormatInt(int64(config.Port), 10))
	listener, err := d.binder.Listen("tcp", hostport)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc(config.WebEndPoint, d.serveRoot)
	mux.HandleFunc(config.LogEndPoint, d.serveLogs)
	d.server = &http.Server{
		Addr:    hostport,
		Handler: mux,
	}
	d.wg.Add(1)
	go d.serve(listener)

	return d, nil
}

func reader(wsconn *websocket.Conn) {
	defer wsconn.Close() // client is gone

	wsconn.SetReadLimit(512)
	wsconn.SetReadDeadline(time.Now().Add(pongWait))
	wsconn.SetPongHandler(func(string) error {
		wsconn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, _, err := wsconn.ReadMessage()
		if err != nil {
			return
		}
	}
}

func (d *WebsocketServerDestination) serveLogs(w http.ResponseWriter, r *http.Request) {
	// a new websocket client is connected
	d.logger.Debug("New websocket connection for logs")
	d.mu.Lock()
	wsconn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			d.logger.Error("Websocket handshake error", "error", err)
		} else {
			d.logger.Error("Websocket upgrade error", "error", err)
		}
		d.mu.Unlock()
		return
	}
	d.logger.Debug("Connection upgraded to websocket")
	d.connections[wsconn] = true
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		delete(d.connections, wsconn)
		d.mu.Unlock()
	}()

	d.wg.Add(1)
	go d.writeLogs(wsconn)
	reader(wsconn)
}

func (d *WebsocketServerDestination) writeLogs(wsconn *websocket.Conn) (err error) {
	pingTicker := time.NewTicker(pingPeriod)
	defer func() {
		pingTicker.Stop()
		if err == nil {
			err = wsconn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye!"),
				time.Now().Add(time.Second),
			)
		}
		wsconn.Close()
		if err != nil {
			d.logger.Info("Websocket connection closed", "error", err)
		} else {
			d.logger.Info("Websocket connection closed")
		}
		d.wg.Done()
	}()

	var message *model.FullMessage
	var ok bool
	var writer io.WriteCloser

	for {
		select {
		case <-pingTicker.C:
			wsconn.SetWriteDeadline(time.Now().Add(writeWait))
			err = wsconn.WriteMessage(websocket.PingMessage, []byte{})
			if err != nil {
				return err
			}
		case <-d.stopchan:
			// server is shutting down
			return nil
		case message, ok = <-d.sendQueue:
			if !ok || message == nil {
				// no more messages to send
				d.dofatal()
				return nil
			}
			uid := message.Uid
			if writer == nil {
				writer, err = wsconn.NextWriter(d.messageType)
				if err != nil {
					// client is gone
					d.nack(uid)
					return err
				}
			}

			wsconn.SetWriteDeadline(time.Now().Add(writeWait))
			err = d.encoder(message, writer)
			model.Free(message.Fields)
			if err == nil {
				// flush the ws buffer
				err = writer.Close()
				writer = nil
				if err == nil {
					d.ack(uid)
				} else {
					// error when flushing
					d.nack(uid)
					return err
				}
			} else if encoders.IsEncodingError(err) {
				// message can not be encoded
				d.permerr(uid)
			} else {
				// error writing to client, must be gone
				d.nack(uid)
				return err
			}
		}
	}
}

func (d *WebsocketServerDestination) serveRoot(w http.ResponseWriter, r *http.Request) {
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()
	w.WriteHeader(http.StatusOK)
}

func (d *WebsocketServerDestination) serve(listener net.Listener) (err error) {
	defer func() {
		d.dofatal()
		if err != nil {
			d.logger.Info("Websocket server has stopped", "error", err)
		} else {
			d.logger.Info("Websocket server has stopped")
		}
		listener.Close()
		d.wg.Done()
	}()
	return d.server.Serve(listener)
}

func (d *WebsocketServerDestination) Close() (err error) {
	// Send will not be called again, we can close the sendQueue
	close(d.sendQueue)
	// close the HTTP server
	err = d.server.Close()
	// wait that the webserver and the websocket connections have finished
	d.wg.Wait()
	// disconnect everything
	d.mu.Lock()
	for wsconn := range d.connections {
		wsconn.Close()
	}
	d.connections = make(map[*websocket.Conn]bool)
	d.mu.Unlock()
	// nack remaining messages
	for message := range d.sendQueue {
		d.nack(message.Uid)
	}
	return err
}

func (d *WebsocketServerDestination) Send(msg *model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	select {
	case d.sendQueue <- msg:
		return nil
	case <-d.stopchan:
		return fmt.Errorf("Websocket server destination is shutting down")
	}
}

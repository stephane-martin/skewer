package store

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
)

var OPEN = []byte("relp_version=0\nrelp_software=skewer\ncommands=syslog")
var endl = []byte("\n")

type relpDestination struct {
	logger      log15.Logger
	fatal       chan struct{}
	registry    *prometheus.Registry
	conn        net.Conn
	scanner     *bufio.Scanner
	once        sync.Once
	format      string
	ack         storeCallback
	nack        storeCallback
	permerr     storeCallback
	curtxnr     uint64
	txnr2msgid  sync.Map
	window      chan bool
	relpTimeout time.Duration
	encoder     model.Encoder
}

func NewRelpDestination(ctx context.Context, bc conf.BaseConfig, ack, nack, permerr storeCallback, logger log15.Logger) (dest Destination, err error) {

	d := &relpDestination{
		logger:      logger,
		registry:    prometheus.NewRegistry(),
		ack:         ack,
		nack:        nack,
		permerr:     permerr,
		format:      bc.RelpDest.Format,
		window:      make(chan bool, bc.RelpDest.WindowSize),
		relpTimeout: bc.RelpDest.RelpTimeout,
		fatal:       make(chan struct{}),
	}

	defer func() {
		if d.conn != nil && err != nil {
			d.conn.Close()
		}
	}()

	//d.registry.MustRegister(d.ackCounter)

	path := strings.TrimSpace(bc.RelpDest.UnixSocketPath)
	if len(path) == 0 {
		addr := fmt.Sprintf("%s:%d", bc.RelpDest.Host, bc.RelpDest.Port)
		c, err := net.DialTimeout("tcp", addr, bc.RelpDest.ConnTimeout)
		if err != nil {
			logger.Error("Error connecting on TCP", "error", err)
			return nil, err
		}
		tcpconn := c.(*net.TCPConn)
		tcpconn.SetNoDelay(true)
		if bc.RelpDest.KeepAlive {
			tcpconn.SetKeepAlive(true)
			tcpconn.SetKeepAlivePeriod(bc.RelpDest.KeepAlivePeriod)
		}
		d.conn = tcpconn
	} else {
		d.conn, err = net.DialTimeout("unix", bc.RelpDest.UnixSocketPath, bc.RelpDest.ConnTimeout)
		if err != nil {
			logger.Error("Error connecting to unix socket", "error", err)
			return nil, err
		}
	}

	d.encoder, err = model.NewEncoder(d.conn, d.format)
	if err != nil {
		return nil, err
	}

	d.scanner = bufio.NewScanner(d.conn)
	d.scanner.Split(utils.RelpSplit)

	err = d.wopen()
	if err != nil {
		return nil, err
	}
	d.conn.SetReadDeadline(time.Now().Add(bc.RelpDest.ConnTimeout))
	txnr, retcode, _, err := d.scan()
	if err != nil {
		return nil, err
	}
	if txnr != 0 {
		return nil, fmt.Errorf("RELP server answered 'open' with a non-zero txnr: '%d'", txnr)
	}
	if retcode != 200 {
		return nil, fmt.Errorf("RELP server answered 'open' with a non-200 status code: '%d'", retcode)
	}

	rebind := bc.RelpDest.Rebind
	if rebind > 0 {
		go func() {
			select {
			case <-ctx.Done():
			case <-time.After(rebind):
				logger.Info("RELP destination rebind period has expired", "rebind", rebind.String())
				d.once.Do(func() { close(d.fatal) })
			}
		}()
	}

	go d.handleRspAnswers()

	return d, nil
}

func (d *relpDestination) handleRspAnswers() {
	for {
		d.conn.SetReadDeadline(time.Now().Add(d.relpTimeout))
		txnr, retcode, _, err := d.scan()
		if err != nil {
			d.logger.Info("RELP destination read error", "error", err)
			d.once.Do(func() { close(d.fatal) })
			return
		}
		uid, ok := d.txnr2msgid.Load(txnr)
		if ok {
			d.txnr2msgid.Delete(txnr)
			<-d.window
			if retcode == 200 {
				d.ack(uid.([16]byte))
			} else {
				d.nack(uid.([16]byte))
				d.logger.Info("RELP server returned a non-200 code", "code", retcode)
				d.once.Do(func() { close(d.fatal) })
			}
		}
	}
}

func (d *relpDestination) scan() (txnr uint64, retcode int, data []byte, err error) {
	if !d.scanner.Scan() {
		err = d.scanner.Err()
		return
	}
	splits := bytes.SplitN(d.scanner.Bytes(), sp, 4)
	txnr, _ = strconv.ParseUint(string(splits[0]), 10, 64)
	if string(splits[1]) != "rsp" {
		err = fmt.Errorf("RELP server answered with invalid command: '%s'", string(splits[1]))
		return
	}
	datalen, _ := strconv.Atoi(string(splits[2]))
	if datalen == 0 {
		data = []byte{}
		return
	}
	data = bytes.Trim(splits[3], " \r\n")
	if len(data) >= 3 {
		code := string(data[:3])
		if code == "200" {
			retcode = 200
		} else if code == "500" {
			retcode = 500
		}
	}
	return
}

func (d *relpDestination) w(command string, v interface{}) (txnr uint64, err error) {
	txnr = atomic.AddUint64(&d.curtxnr, 1)
	return txnr, model.FrameEncode(d.encoder, endl, txnr, sp, command, sp, v)
}

func (d *relpDestination) wopen() (err error) {
	return model.ChainEncode(d.encoder, int(0), sp, "open", sp, len(OPEN), sp, OPEN, endl)
}

func (d *relpDestination) wclose() (err error) {
	_, err = d.w("close", nil)
	return
}

func (d *relpDestination) Send(message model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	var txnr uint64
	txnr, err = d.w("syslog", &message)
	if err == nil {
		d.window <- true
		d.txnr2msgid.Store(txnr, message.Uid)
	} else if model.IsEncodingError(err) {
		d.permerr(message.Uid)
	} else {
		d.nack(message.Uid)
		d.once.Do(func() { close(d.fatal) })
	}
	return
}

func (d *relpDestination) Close() {
	d.wclose()
	d.conn.SetReadDeadline(time.Now().Add(time.Second))
	d.scan()
	d.conn.Close()
	// NACK all pending messages
	d.txnr2msgid.Range(func(k interface{}, v interface{}) bool {
		d.nack(v.([16]byte))
		return true
	})
}

func (d *relpDestination) Fatal() chan struct{} {
	return d.fatal
}

func (d *relpDestination) Gather() ([]*dto.MetricFamily, error) {
	return d.registry.Gather()
}

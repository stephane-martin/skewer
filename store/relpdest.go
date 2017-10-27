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
	logger     log15.Logger
	fatal      chan struct{}
	registry   *prometheus.Registry
	conn       net.Conn
	scanner    *bufio.Scanner
	once       sync.Once
	format     string
	ack        storeCallback
	nack       storeCallback
	permerr    storeCallback
	curtxnr    uint64
	txnr2msgid sync.Map
	window     chan bool
	answers    chan bool
}

func NewRelpDestination(ctx context.Context, bc conf.BaseConfig, ack, nack, permerr storeCallback, logger log15.Logger) (dest Destination, err error) {

	d := &relpDestination{
		logger:   logger,
		registry: prometheus.NewRegistry(),
		ack:      ack,
		nack:     nack,
		permerr:  permerr,
		format:   bc.RelpDest.Format,
		window:   make(chan bool, bc.RelpDest.WindowSize),
		answers:  make(chan bool),
	}

	defer func() {
		if d.conn != nil && err != nil {
			d.conn.Close()
		}
	}()

	//d.registry.MustRegister(d.ackCounter)

	d.fatal = make(chan struct{})

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

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(bc.RelpDest.RelpTimeout):
				d.once.Do(func() { close(d.fatal) })
				return
			case <-d.answers:
			}
		}
	}()

	go d.getanswers()

	return d, nil
}

func (d *relpDestination) getanswers() {
	for {
		txnr, retcode, _, err := d.scan()
		if err != nil {
			d.once.Do(func() { close(d.fatal) })
			return
		}
		d.answers <- true
		uid, ok := d.txnr2msgid.Load(txnr)
		if ok {
			if retcode == 200 {
				d.ack(uid.([16]byte))
			} else {
				d.nack(uid.([16]byte))
			}
			<-d.window
			d.txnr2msgid.Delete(txnr)
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

func (d *relpDestination) w(command string, b []byte) (uint64, error) {
	txnr := atomic.AddUint64(&d.curtxnr, 1)
	header := fmt.Sprintf("%d %s %d", txnr, command, len(b))
	if len(b) > 0 {
		return txnr, utils.ChainWrites(d.conn, []byte(header), sp, b, endl)
	}
	return txnr, utils.ChainWrites(d.conn, []byte(header), endl)
}

func (d *relpDestination) wopen() (err error) {
	header := fmt.Sprintf("0 open %d", len(OPEN))
	return utils.ChainWrites(d.conn, []byte(header), sp, OPEN, endl)
}

func (d *relpDestination) wclose() (err error) {
	_, err = d.w("close", nil)
	return
}

func (d *relpDestination) wsyslog(serialized []byte) (uint64, error) {
	return d.w("syslog", serialized)
}

func (d *relpDestination) Send(message *model.TcpUdpParsedMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	var txnr uint64
	var serialized []byte
	serialized, err = message.MarshalAll(d.format)
	if err != nil {
		d.permerr(message.Uid)
		return
	}
	txnr, err = d.wsyslog(serialized)
	if err != nil {
		d.nack(message.Uid)
		d.once.Do(func() { close(d.fatal) })
		return
	}
	d.window <- true
	d.txnr2msgid.Store(txnr, message.Uid)
	return nil
}

func (d *relpDestination) Close() {
	d.wclose()
	d.conn.Close()
}

func (d *relpDestination) Fatal() chan struct{} {
	return d.fatal
}

func (d *relpDestination) Gather() ([]*dto.MetricFamily, error) {
	return d.registry.Gather()
}

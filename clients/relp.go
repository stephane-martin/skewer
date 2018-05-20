package clients

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"math"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/free/concurrent-writer/concurrent"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/encoders"
	"github.com/stephane-martin/skewer/encoders/baseenc"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/stephane-martin/skewer/utils/queue"
	"github.com/zond/gotomic"
	"go.uber.org/atomic"
)

var OPEN = []byte("relp_version=0\nrelp_software=skewer\ncommands=syslog")
var endl = []byte("\n")
var sp = []byte(" ")
var zerotime = time.Time{}

func RELPClientError(err error) error {
	return eerrors.WithTypes(err, "RELPClient")
}

var ErrRELPNotConnected = RELPClientError(eerrors.New("Not connected"))
var ErrRELPClosed = RELPClientError(eerrors.New("Closed"))
var ErrRELPNoHost = RELPClientError(eerrors.New("Empty host or empty unix socket path"))
var ErrRELPNoPort = RELPClientError(eerrors.New("Empty port"))
var ErrRELPTimeout = RELPClientError(eerrors.New("Timeout waiting for RELP response"))

type IntKey int32

func (self IntKey) HashCode() uint32 {
	return uint32(self)
}
func (self IntKey) Equals(t gotomic.Thing) bool {
	if ik, ok := t.(IntKey); ok {
		return int32(self) == int32(ik)
	}
	return false
}

type Txnr2UidMap struct {
	h   *gotomic.Hash
	sem *utils.Semaphore
}

func NewTxnrMap(maxsize int32) *Txnr2UidMap {
	if maxsize == 0 {
		maxsize = 128
	}
	m := Txnr2UidMap{
		sem: utils.NewSemaphore(maxsize),
		h:   gotomic.NewHash(),
	}
	return &m
}

func (m *Txnr2UidMap) Put(txnr int32, uid utils.MyULID) (err error) {
	// if there is enough room in m, put (txnr, uid)
	// if not, wait for some room
	// can be interrupted, in that case, return error
	if err = m.sem.Acquire(); err != nil {
		return err
	}
	if _, overw := m.h.Put(IntKey(txnr), uid); overw {
		m.sem.Release()
	}
	return nil
}

func (m *Txnr2UidMap) Get(txnr int32) (uid utils.MyULID, err error) {
	// get the uid for the given txnr
	// if found, delete (uid, txnr) from m
	// if not found, return error
	if t, present := m.h.Delete(IntKey(txnr)); present {
		m.sem.Release()
		return t.(utils.MyULID), nil
	}
	return uid, RELPClientError(eerrors.Errorf("Unknown TXNR: %d", txnr))
}

func (m *Txnr2UidMap) ForEach(f func(int32, utils.MyULID)) {
	g := func(k gotomic.Hashable, v gotomic.Thing) bool {
		f(int32(k.(IntKey)), v.(utils.MyULID))
		return false
	}
	m.h.Each(g)
}

func (m *Txnr2UidMap) Dispose() {
	m.sem.Dispose()
}

type RELPClient struct {
	host            string
	port            int
	path            string
	format          baseenc.Format
	keepAlive       bool
	keepAlivePeriod time.Duration
	connTimeout     time.Duration
	flushPeriod     time.Duration
	tlsConfig       *tls.Config

	relpTimeout time.Duration

	conn    net.Conn
	writer  *concurrent.Writer
	encoder encoders.Encoder
	scanner *bufio.Scanner
	logger  log15.Logger
	ticker  *time.Ticker

	curtxnr    atomic.Int32
	txnr2msgid *Txnr2UidMap
	windowSize int32

	ackChan  *queue.AckQueue
	nackChan *queue.AckQueue

	handleWg sync.WaitGroup

	closed atomic.Bool
}

func NewRELPClient(logger log15.Logger) *RELPClient {
	return &RELPClient{logger: logger.New("clientkind", "RELP")}
}

func (c *RELPClient) Host(host string) *RELPClient {
	// TODO: support multiple hosts as failovers
	c.host = host
	return c
}

func (c *RELPClient) Port(port int) *RELPClient {
	c.port = port
	return c
}

func (c *RELPClient) Path(path string) *RELPClient {
	c.path = path
	return c
}

func (c *RELPClient) Format(format baseenc.Format) *RELPClient {
	c.format = format
	return c
}

func (c *RELPClient) KeepAlive(keepAlive bool) *RELPClient {
	c.keepAlive = keepAlive
	return c
}

func (c *RELPClient) KeepAlivePeriod(period time.Duration) *RELPClient {
	c.keepAlivePeriod = period
	return c
}

func (c *RELPClient) ConnTimeout(timeout time.Duration) *RELPClient {
	c.connTimeout = timeout
	return c
}

func (c *RELPClient) RelpTimeout(timeout time.Duration) *RELPClient {
	c.relpTimeout = timeout
	return c
}

func (c *RELPClient) WindowSize(size int32) *RELPClient {
	c.windowSize = size
	return c
}

func (c *RELPClient) FlushPeriod(period time.Duration) *RELPClient {
	c.flushPeriod = period
	return c
}

func (c *RELPClient) TLS(config *tls.Config) *RELPClient {
	c.tlsConfig = config
	return c
}

func (c *RELPClient) Connect() (err error) {
	if c.closed.Load() {
		return ErrRELPClosed
	}
	defer func() {
		if err != nil {
			c.conn = nil
			c.writer = nil
			c.ackChan = nil
			c.nackChan = nil
			c.scanner = nil
		}
	}()

	if c.conn != nil {
		return nil
	}

	c.encoder, err = encoders.GetEncoder(c.format)
	if err != nil {
		return err
	}

	var conn net.Conn

	if len(c.path) == 0 {
		if len(c.host) == 0 {
			return ErrRELPNoHost
		}
		if c.port == 0 {
			return ErrRELPNoPort
		}
		hostport := net.JoinHostPort(c.host, strconv.FormatInt(int64(c.port), 10))
		var dialer *net.Dialer
		if c.connTimeout == 0 {
			dialer = &net.Dialer{}
		} else {
			dialer = &net.Dialer{Timeout: c.connTimeout}
		}
		if c.tlsConfig == nil {
			conn, err = dialer.Dial("tcp", hostport)
		} else {
			conn, err = tls.DialWithDialer(dialer, "tcp", hostport, c.tlsConfig)
		}
		if err != nil {
			return RELPClientError(eerrors.Wrap(err, "Error connecting to TCP server"))
		}
		tcpconn := conn.(*net.TCPConn)
		if c.keepAlive {
			_ = tcpconn.SetKeepAlive(true)
			_ = tcpconn.SetKeepAlivePeriod(c.keepAlivePeriod)
		}
	} else {
		if c.connTimeout == 0 {
			conn, err = net.Dial("unix", c.path)
		} else {
			conn, err = net.DialTimeout("unix", c.path, c.connTimeout)
		}
		if err != nil {
			return RELPClientError(eerrors.Wrap(err, "Error connecting to TCP server"))
		}
	}

	c.conn = conn
	c.scanner = bufio.NewScanner(c.conn)
	c.scanner.Split(utils.RelpSplit)

	err = c.wopen()
	if err != nil {
		return RELPClientError(eerrors.Wrap(err, "Error opening RELP session"))
	}
	if c.connTimeout != 0 {
		_ = c.conn.SetReadDeadline(time.Now().Add(c.connTimeout))
	}
	txnr, retcode, _, err := c.scan()
	if err != nil {
		return err
	}
	if txnr != 0 {
		return RELPClientError(eerrors.Errorf("RELP server answered 'open' with a non-zero txnr: '%d'", txnr))
	}
	if retcode != 200 {
		return RELPClientError(eerrors.Errorf("RELP server answered 'open' with a non-200 status code: '%d'", retcode))
	}
	if c.flushPeriod > 0 {
		c.writer = concurrent.NewWriterAutoFlush(c.conn, 4096, 0.75)
		c.ticker = time.NewTicker(c.flushPeriod)
		go func() {
			for range c.ticker.C {
				err := c.Flush()
				if err != nil {
					if eerrors.HasBrokenPipe(err) || eerrors.HasFileClosed(err) {
						c.logger.Warn("Broken pipe detected when flushing buffers", "error", err)
						_ = c.conn.Close()
						return
					}
					if eerrors.IsTimeout(err) {
						c.logger.Warn("Timeout detected when flushing buffers", "error", err)
						_ = c.conn.Close()
						return
					}
					c.logger.Warn("Unexpected error flushing buffers", "error", err)
				}
			}
		}()
	} else {
		c.writer = nil
		c.ticker = nil
	}
	var window int32 = 128
	if c.windowSize > 0 {
		window = c.windowSize
	}
	c.ackChan = queue.NewAckQueue()
	c.nackChan = queue.NewAckQueue()
	c.txnr2msgid = NewTxnrMap(window)
	c.handleWg.Add(1)
	go func() {
		defer c.handleWg.Done()
		err := c.handleRspAnswers()
		if err != nil {
			c.logger.Info(err.Error())
		}
	}()
	return nil
}

func (c *RELPClient) encode(command string, v interface{}) (buf string, txnr int32, err error) {
	// first encode the message
	buf, err = encoders.ChainEncode(c.encoder, v)
	if err != nil {
		return "", 0, err
	}
	// if no error, we can increment txnr
	txnr = c.curtxnr.Add(1)
	if len(buf) == 0 {
		buf, err = encoders.RELPEncode(c.encoder, txnr, command, nil) // cannot fail
	} else {
		buf, err = encoders.RELPEncode(c.encoder, txnr, command, buf) // cannot fail
	}
	if err != nil {
		c.logger.Error("RELPEncode error, should not happen", "error", err)
		return "", 0, RELPClientError(eerrors.Wrap(err, "RELPEncode error"))
	}
	return buf, txnr, nil
}

func (c *RELPClient) wopen() (err error) {
	if c.conn == nil {
		return ErrRELPNotConnected
	}
	var buf string
	buf, err = encoders.ChainEncode(c.encoder, int(0), sp, "open", sp, len(OPEN), sp, OPEN, endl)
	if err != nil {
		return err
	}
	_, err = io.WriteString(c.conn, buf)
	return err
}

func (c *RELPClient) wclose() (err error) {
	if c.conn == nil {
		return ErrRELPNotConnected
	}
	buf, _, _ := c.encode("close", nil)
	_, err = io.WriteString(c.conn, buf)
	return err
}

func (c *RELPClient) scan() (txnr int32, retcode int, data []byte, err error) {
	ret, err := utils.ScanRecover(c.scanner)
	if err != nil {
		return 0, 0, nil, RELPClientError(err)
	}
	if !ret {
		return 0, 0, nil, c.scanner.Err()
	}
	splits := bytes.SplitN(c.scanner.Bytes(), sp, 4)
	txnr64, _ := strconv.ParseInt(string(splits[0]), 10, 64)
	if txnr64 > int64(math.MaxInt32) {
		return 0, 0, nil, RELPClientError(eerrors.Errorf("RELPClient: received txnr is not an int32: %d", txnr64))
	}
	if string(splits[1]) != "rsp" {
		return 0, 0, nil, RELPClientError(eerrors.Errorf("RELP server answered with invalid command: '%s'", string(splits[1])))
	}
	txnr = int32(txnr64)
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

func (c *RELPClient) handleRspAnswers() error {
	// returns if
	// - Close() was called (hence the conn was closed)
	// - conn was closed by server
	// - there was a RELP session timeout
	defer func() {
		_ = c.conn.Close() // in case the conn was not properly closed
		// now we can NACK the messages that we did not have a response for,
		// as no more txnr2msgid entries will be added by Send()
		keys := make([]int32, 0)
		c.txnr2msgid.ForEach(
			func(txnr int32, uid utils.MyULID) {
				c.nackChan.Put(uid, conf.RELP)
				keys = append(keys, txnr)
			},
		)
		// clear txnr2msgid
		for _, k := range keys {
			c.txnr2msgid.Get(k)
		}
		c.txnr2msgid.Dispose()
		// close the ackChan channels: we have nothing more to say
		c.ackChan.Dispose()
		c.nackChan.Dispose()
	}()

	for {
		if c.relpTimeout > 0 {
			_ = c.conn.SetReadDeadline(time.Now().Add(c.relpTimeout))
		}
		txnr, retcode, _, err := c.scan()
		_ = c.conn.SetReadDeadline(zerotime) // disable deadline

		if err != nil {
			if eerrors.HasFileClosed(err) || eerrors.HasBrokenPipe(err) {
				return ErrRELPClosed
			}
			if eerrors.IsTimeout(err) {
				return ErrRELPTimeout
			}
			if eerrors.IsTemporary(err) {
				c.logger.Info("RELP client temporary error", "error", err)
				continue
			}
			return RELPClientError(eerrors.Wrap(err, "Error scanning RELP server response"))
		}
		uid, err := c.txnr2msgid.Get(txnr)
		if err != nil {
			c.logger.Warn("RELP CLient: unknown txnr", "txnr", txnr)
			continue
		}
		if retcode != 200 {
			c.nackChan.Put(uid, conf.RELP)
			continue
		}
		c.ackChan.Put(uid, conf.RELP)
	}
}

func (c *RELPClient) doSendOne(msg *model.FullMessage) (err error) {
	if msg == nil {
		return nil
	}
	buf, txnr, err := c.encode("syslog", msg)
	if err != nil {
		return err
	}
	if len(buf) == 0 {
		// nothing to do
		c.ackChan.Put(msg.Uid, conf.RELP)
		return nil
	}
	if c.writer == nil {
		_, err = io.WriteString(c.conn, buf)
	} else {
		_, err = c.writer.WriteString(buf)
	}
	if err != nil {
		return err
	}
	// everything alright, we register the Uid to wait for the server response
	return c.txnr2msgid.Put(txnr, msg.Uid)
}

func (c *RELPClient) Send(ctx context.Context, msg *model.FullMessage) error {
	if c.conn == nil {
		return ErrRELPNotConnected
	}
	if c.closed.Load() {
		return ErrRELPClosed
	}
	return RELPClientError(eerrors.Wrap(c.doSendOne(msg), "Error sending RELP message"))
}

func (c *RELPClient) Close() (err error) {
	if !c.closed.CAS(false, true) {
		// already closed
		return nil
	}
	if c.conn == nil {
		return ErrRELPNotConnected
	}
	if c.ticker != nil {
		c.ticker.Stop()
	}
	// wait that pending Send() have expired
	c.conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
	c.Flush()
	time.Sleep(500 * time.Millisecond)

	// try to notify the server
	c.conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
	err = c.wclose()

	// close the connection to the RELP server
	_ = c.conn.Close() // makes handleRspAnswers return
	c.handleWg.Wait()
	return RELPClientError(eerrors.Wrap(err, "Error closing RELP session"))
}

func (c *RELPClient) Ack() *queue.AckQueue {
	return c.ackChan
}

func (c *RELPClient) Nack() *queue.AckQueue {
	return c.nackChan
}

func (c *RELPClient) Flush() error {
	if c.writer != nil {
		return RELPClientError(eerrors.Wrap(c.writer.Flush(), "Error flushing RELP connection buffer"))
	}
	return nil
}

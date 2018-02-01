package clients

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"math"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/free/concurrent-writer/concurrent"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/model/encoders"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/queue"
	"github.com/stephane-martin/skewer/utils/queue/message"
	"github.com/zond/gotomic"
)

var OPEN = []byte("relp_version=0\nrelp_software=skewer\ncommands=syslog")
var endl = []byte("\n")
var sp = []byte(" ")

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
	err = fmt.Errorf("unknown txnr: %d", txnr)
	return
}

type Iterator func(int32, utils.MyULID)

func (m *Txnr2UidMap) ForEach(f Iterator) {
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
	format          encoders.Format
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

	curtxnr    int32
	txnr2msgid *Txnr2UidMap
	windowSize int32

	ackChan   *queue.AckQueue
	nackChan  *queue.AckQueue
	sendQueue *message.Ring

	sync.Mutex
	sendWg   sync.WaitGroup
	handleWg sync.WaitGroup
}

func NewRELPClient(logger log15.Logger) *RELPClient {
	return &RELPClient{logger: logger.New("clientkind", "RELP")}
}

func (c *RELPClient) Host(host string) *RELPClient {
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

func (c *RELPClient) Format(format encoders.Format) *RELPClient {
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
	c.Lock()
	defer func() {
		if err != nil {
			c.conn = nil
			c.writer = nil
			c.ackChan = nil
			c.nackChan = nil
			c.scanner = nil
			c.sendQueue = nil
		}
		c.Unlock()
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
			return fmt.Errorf("RELPClient: specify a host or a unix path")
		}
		if c.port == 0 {
			return fmt.Errorf("RELPClient: specify a port")
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
			return err
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
			return err
		}
	}

	c.conn = conn
	c.scanner = bufio.NewScanner(c.conn)
	c.scanner.Split(utils.RelpSplit)

	err = c.wopen()
	if err != nil {
		return err
	}
	if c.connTimeout != 0 {
		_ = c.conn.SetReadDeadline(time.Now().Add(c.connTimeout))
	}
	txnr, retcode, _, err := c.scan()
	if err != nil {
		return err
	}
	if txnr != 0 {
		return fmt.Errorf("RELP server answered 'open' with a non-zero txnr: '%d'", txnr)
	}
	if retcode != 200 {
		return fmt.Errorf("RELP server answered 'open' with a non-200 status code: '%d'", retcode)
	}
	if c.flushPeriod > 0 {
		c.writer = concurrent.NewWriterAutoFlush(c.conn, 4096, 0.75)
		c.ticker = time.NewTicker(c.flushPeriod)
		go func() {
			var err error
			for range c.ticker.C {
				err = c.Flush()
				if err != nil {
					if utils.IsBrokenPipe(err) || utils.IsFileClosed(err) {
						c.logger.Warn("Broken pipe detected when flushing buffers", "error", err)
						_ = c.conn.Close()
						c.sendQueue.Dispose()
						return
					}
					if utils.IsTimeout(err) {
						c.logger.Warn("Timeout detected when flushing buffers", "error", err)
						_ = c.conn.Close()
						c.sendQueue.Dispose()
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
	c.sendQueue = message.NewRing(uint64(window))
	c.txnr2msgid = NewTxnrMap(window)
	c.handleWg.Add(1)
	go c.handleRspAnswers()
	c.sendWg.Add(1)
	go c.doSend()
	return nil
}

func (c *RELPClient) encode(command string, v interface{}) (buf []byte, txnr int32, err error) {
	// first encode the message
	buf, err = encoders.ChainEncode(c.encoder, v)
	if err != nil {
		return nil, 0, err
	}
	// if no error, we can increment txnr
	txnr = atomic.AddInt32(&c.curtxnr, 1)
	if len(buf) == 0 {
		buf, err = encoders.RelpEncode(c.encoder, txnr, command, nil) // cannot fail
	} else {
		buf, err = encoders.RelpEncode(c.encoder, txnr, command, buf) // cannot fail
	}
	if err != nil {
		c.logger.Error("RelpEncode error, should not happen", "error", err)
		return nil, 0, err
	}
	return buf, txnr, nil
}

func (c *RELPClient) wopen() (err error) {
	var buf []byte
	buf, err = encoders.ChainEncode(c.encoder, int(0), sp, "open", sp, len(OPEN), sp, OPEN, endl)
	if err != nil {
		return err
	}
	_, err = c.conn.Write(buf)
	return err
}

func (c *RELPClient) wclose() (err error) {
	buf, _, _ := c.encode("close", nil)
	_, err = c.conn.Write(buf)
	return err
}

func (c *RELPClient) scan() (txnr int32, retcode int, data []byte, err error) {
	if !c.scanner.Scan() {
		err = c.scanner.Err()
		return
	}
	splits := bytes.SplitN(c.scanner.Bytes(), sp, 4)
	txnr64, _ := strconv.ParseInt(string(splits[0]), 10, 64)
	if txnr64 > int64(math.MaxInt32) {
		err = fmt.Errorf("RELPClient: received txnr is not an int32")
		return
	}
	if string(splits[1]) != "rsp" {
		err = fmt.Errorf("RELP server answered with invalid command: '%s'", string(splits[1]))
		return
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

func (c *RELPClient) Send(msg *model.FullMessage) error {
	// returns with error if the sendQueue has been disposed
	return c.sendQueue.Put(msg)
}

func (c *RELPClient) handleRspAnswers() {
	defer func() {
		// we arrive here if
		// - Close() was called (hence the conn was closed)
		// - conn was closed by server
		// - there was a RELP session timeout
		c.sendQueue.Dispose() // stop stashing messages to be sent
		_ = c.conn.Close()    // in case the conn was not properly closed
		// the closed conn and the disposed queue will make doSend to return eventually
		c.sendWg.Wait() // wait for doSend to return
		// now we can NACK the messages that we did not have a response for,
		// as no more txnr2msgid entries will be added by doSend
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
		c.handleWg.Done()
	}()
	for {
		if c.relpTimeout > 0 {
			_ = c.conn.SetReadDeadline(time.Now().Add(c.relpTimeout))
		}
		txnr, retcode, _, err := c.scan()
		_ = c.conn.SetReadDeadline(time.Time{})
		if utils.IsFileClosed(err) {
			// connection is closed
			c.logger.Debug("Connection has been closed")
			return
		} else if err != nil {
			if nerr, ok := err.(net.Error); ok {
				if nerr.Timeout() {
					c.logger.Info("timeout waiting for RELP answers", "error", err)
					return
				} else if nerr.Temporary() {
					c.logger.Info("temporary error", "error", err)
					continue
				}
			}
			c.logger.Info("error reading server responses", "error", err)
			return
		}
		uid, err := c.txnr2msgid.Get(txnr)
		if err == nil {
			if retcode == 200 {
				c.ackChan.Put(uid, conf.RELP)
			} else {
				c.nackChan.Put(uid, conf.RELP)
			}
		} else {
			c.logger.Warn("Unknown txnr", "txnr", txnr)
		}
	}
}

func (c *RELPClient) doSendOne(msg *model.FullMessage) (err error) {
	if msg == nil {
		return
	}
	buf, txnr, err := c.encode("syslog", msg)
	if err != nil {
		return encoders.NonEncodableError
	}
	if len(buf) == 0 {
		c.ackChan.Put(msg.Uid, conf.RELP)
		return nil
	}
	if c.writer == nil {
		_, err = c.conn.Write(buf)
	} else {
		_, err = c.writer.Write(buf)
	}
	if err == nil {
		return c.txnr2msgid.Put(txnr, msg.Uid)
	}
	c.nackChan.Put(msg.Uid, conf.RELP)
	return err
}

func (c *RELPClient) doSend() {
	defer func() {
		c.sendQueue.Dispose()
		_ = c.Flush()
		for {
			msg, err := c.sendQueue.Get()
			if err != nil {
				break
			}
			c.nackChan.Put(msg.Uid, conf.RELP)
		}
		c.sendWg.Done()
	}()
	for {
		msg, err := c.sendQueue.Get()
		if err == utils.ErrDisposed {
			c.logger.Debug("the queue has been disposed")
			return
		} else if err != nil {
			c.logger.Info("unexpected error getting message from queue", "error", err)
			return
		}

		err = c.doSendOne(msg)
		model.FullFree(msg) // msg can be reused from here

		if err == utils.ErrDisposed {
			c.logger.Debug("the queue has been disposed")
			return
		} else if encoders.IsEncodingError(err) {
			c.logger.Warn("dropped non-encodable message")
			continue
		} else if err != nil {
			if utils.IsBrokenPipe(err) {
				c.logger.Info("broken pipe writing to server")
				return
			}
			c.logger.Info("error writing message to the server", "error", err)
			continue
		}
	}
}

func (c *RELPClient) Close() (err error) {
	c.Lock()
	defer c.Unlock()
	if c.conn == nil {
		return nil
	}
	if c.ticker != nil {
		c.ticker.Stop()
	}
	c.sendQueue.Dispose()
	c.conn.SetWriteDeadline(time.Now().Add(time.Second))
	c.sendWg.Wait()
	err = c.wclose() // try to notify the server
	// close the connection to the RELP server
	_ = c.conn.Close() // makes handleRspAnswers return
	c.handleWg.Wait()
	c.conn = nil
	return err
}

func (c *RELPClient) Ack() *queue.AckQueue {
	return c.ackChan
}

func (c *RELPClient) Nack() *queue.AckQueue {
	return c.nackChan
}

func (c *RELPClient) Flush() error {
	if c.writer != nil {
		return c.writer.Flush()
	}
	return nil
}

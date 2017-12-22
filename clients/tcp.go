package clients

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/free/concurrent-writer/concurrent"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/queue"
)

type SyslogTCPClient struct {
	host            string
	port            int
	path            string
	format          string
	keepAlive       bool
	keepAlivePeriod time.Duration
	connTimeout     time.Duration
	flushPeriod     time.Duration
	tlsConfig       *tls.Config

	lineFraming    bool
	frameDelimiter uint8

	conn    net.Conn
	writer  *concurrent.Writer
	encoder model.Encoder
	ticker  *time.Ticker
	logger  log15.Logger

	errorFlag int32
	errorPrev error

	sync.Mutex
}

func NewSyslogTCPClient(logger log15.Logger) *SyslogTCPClient {
	return &SyslogTCPClient{logger: logger.New("clientkind", "TCP")}
}

func (c *SyslogTCPClient) Host(host string) *SyslogTCPClient {
	c.host = host
	return c
}

func (c *SyslogTCPClient) Port(port int) *SyslogTCPClient {
	c.port = port
	return c
}

func (c *SyslogTCPClient) Path(path string) *SyslogTCPClient {
	c.path = path
	return c
}

func (c *SyslogTCPClient) Format(format string) *SyslogTCPClient {
	c.format = format
	return c
}

func (c *SyslogTCPClient) KeepAlive(keepAlive bool) *SyslogTCPClient {
	c.keepAlive = keepAlive
	return c
}

func (c *SyslogTCPClient) KeepAlivePeriod(period time.Duration) *SyslogTCPClient {
	c.keepAlivePeriod = period
	return c
}

func (c *SyslogTCPClient) ConnTimeout(timeout time.Duration) *SyslogTCPClient {
	c.connTimeout = timeout
	return c
}

func (c *SyslogTCPClient) LineFraming(framing bool) *SyslogTCPClient {
	c.lineFraming = framing
	return c
}

func (c *SyslogTCPClient) FrameDelimiter(delimiter uint8) *SyslogTCPClient {
	c.frameDelimiter = delimiter
	return c
}

func (c *SyslogTCPClient) FlushPeriod(period time.Duration) *SyslogTCPClient {
	c.flushPeriod = period
	return c
}

func (c *SyslogTCPClient) TLS(config *tls.Config) *SyslogTCPClient {
	c.tlsConfig = config
	return c
}

func (c *SyslogTCPClient) Close() (err error) {
	c.Lock()
	defer c.Unlock()
	if c.conn == nil {
		return nil
	}
	if c.ticker != nil {
		c.ticker.Stop()
	}
	err = c.Flush()
	_ = c.conn.Close()
	c.conn = nil
	return
}

func (c *SyslogTCPClient) Connect() (err error) {
	c.Lock()
	defer func() {
		if err != nil {
			c.conn = nil
			c.writer = nil
		}
		c.Unlock()
	}()
	if c.conn != nil {
		return nil
	}

	c.encoder, err = model.NewEncoder(c.format)
	if err != nil {
		return err
	}

	var conn net.Conn

	if len(c.path) == 0 {
		if len(c.host) == 0 {
			return fmt.Errorf("SyslogTCPClient: specify a host or a unix path")
		}
		if c.port == 0 {
			return fmt.Errorf("SyslogTCPClient: specify a port")
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
		_ = tcpconn.SetNoDelay(true)
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
	if c.flushPeriod > 0 {
		c.writer = concurrent.NewWriterAutoFlush(c.conn, 4096, 0.75)
		c.ticker = time.NewTicker(c.flushPeriod)

		go func() {
			var err error
			for range c.ticker.C {
				err = c.Flush()
				if err != nil {
					if utils.IsBrokenPipe(err) {
						_ = c.conn.Close()
						c.logger.Warn("Broken pipe detected when flushing buffers", "error", err)
						c.errorPrev = err
						atomic.StoreInt32(&c.errorFlag, 1)
						return
					}
					if utils.IsTimeout(err) {
						_ = c.conn.Close()
						c.logger.Warn("Timeout detected when flushing buffers", "error", err)
						c.errorPrev = err
						atomic.StoreInt32(&c.errorFlag, 1)
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
	return nil
}

func (c *SyslogTCPClient) Send(msg *model.FullMessage) (err error) {
	// may be called concurrently
	if c.conn == nil {
		return fmt.Errorf("SyslogTCPClient: not connected")
	}
	if msg == nil {
		return nil
	}
	var buf []byte
	if c.lineFraming {
		buf, err = model.ChainEncode(c.encoder, msg, []byte{c.frameDelimiter})
	} else {
		buf, err = model.TcpOctetEncode(c.encoder, msg)
	}
	if err != nil {
		return model.NonEncodableError
	}
	if len(buf) == 0 {
		return nil
	}
	if atomic.LoadInt32(&c.errorFlag) == 1 {
		return c.errorPrev
	}
	if c.writer != nil {
		_, err = c.writer.Write(buf)
	} else {
		_, err = c.conn.Write(buf)
	}
	return err
}

func (c *SyslogTCPClient) Flush() error {
	if c.writer != nil {
		return c.writer.Flush()
	}
	return nil
}

func (c *SyslogTCPClient) Ack() *queue.AckQueue {
	return nil
}

func (c *SyslogTCPClient) Nack() *queue.AckQueue {
	return nil
}

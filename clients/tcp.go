package clients

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/free/concurrent-writer/concurrent"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/encoders"
	"github.com/stephane-martin/skewer/encoders/baseenc"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/queue"
)

type SyslogTCPClient struct {
	host            string
	port            int
	path            string
	format          baseenc.Format
	keepAlive       bool
	keepAlivePeriod time.Duration
	connTimeout     time.Duration
	flushPeriod     time.Duration
	tlsConfig       *tls.Config

	lineFraming    bool
	frameDelimiter uint8

	closed  int32
	conn    net.Conn
	writer  *concurrent.Writer
	encoder encoders.Encoder
	ticker  *time.Ticker
	logger  log15.Logger

	errorFlag int32
	errorPrev error
}

func NewSyslogTCPClient(logger log15.Logger) *SyslogTCPClient {
	return &SyslogTCPClient{logger: logger.New("clientkind", "TCP")}
}

func (c *SyslogTCPClient) Host(host string) *SyslogTCPClient {
	// TODO: support multiple hosts as failovers
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

func (c *SyslogTCPClient) Format(format baseenc.Format) *SyslogTCPClient {
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
	if atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		if c.ticker != nil {
			c.ticker.Stop()
		}
		err = c.Flush()
		if c.conn != nil {
			_ = c.conn.Close()
		}
	}
	return
}

func (c *SyslogTCPClient) Connect(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			c.conn = nil
			c.writer = nil
		}
	}()
	if atomic.LoadInt32(&c.closed) == 1 {
		return errors.New("SyslogTCPClient: closed")
	}
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
			return errors.New("SyslogTCPClient: specify a host or a unix path")
		}
		if c.port == 0 {
			return errors.New("SyslogTCPClient: specify a port")
		}
		hostport := net.JoinHostPort(c.host, strconv.FormatInt(int64(c.port), 10))
		var dialer *net.Dialer
		if c.connTimeout == 0 {
			dialer = &net.Dialer{}
		} else {
			dialer = &net.Dialer{Timeout: c.connTimeout}
		}
		if c.tlsConfig == nil {
			conn, err = dialer.DialContext(ctx, "tcp", hostport)
		} else {
			dialer.Cancel = ctx.Done()
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
					if utils.IsBrokenPipe(err) || utils.IsFileClosed(err) {
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

func (c *SyslogTCPClient) Send(ctx context.Context, msg *model.FullMessage) (err error) {
	if c.conn == nil {
		return errors.New("SyslogTCPClient: not connected")
	}
	if atomic.LoadInt32(&c.closed) == 1 {
		return errors.New("SyslogTCPClient: closed")
	}
	if msg == nil {
		return nil
	}
	var buf string
	if c.lineFraming {
		buf, err = encoders.ChainEncode(c.encoder, msg, []byte{c.frameDelimiter})
	} else {
		buf, err = encoders.TcpOctetEncode(c.encoder, msg)
	}
	if err != nil {
		return encoders.ErrNonEncodable
	}
	if len(buf) == 0 {
		return nil
	}
	if atomic.LoadInt32(&c.errorFlag) == 1 {
		return c.errorPrev
	}
	if c.writer != nil {
		_, err = c.writer.WriteString(buf)
	} else {
		_, err = io.WriteString(c.conn, buf)
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

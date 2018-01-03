package clients

import (
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/model/encoders"
)

type SyslogUDPClient struct {
	host   string
	port   int
	path   string
	format encoders.Format

	conn    net.Conn
	encoder encoders.Encoder
	logger  log15.Logger

	sync.Mutex
}

func NewSyslogUDPClient(logger log15.Logger) *SyslogUDPClient {
	return &SyslogUDPClient{logger: logger.New("clientkind", "UDP")}
}

func (c *SyslogUDPClient) Host(host string) *SyslogUDPClient {
	c.host = host
	return c
}

func (c *SyslogUDPClient) Port(port int) *SyslogUDPClient {
	c.port = port
	return c
}

func (c *SyslogUDPClient) Path(path string) *SyslogUDPClient {
	c.path = path
	return c
}

func (c *SyslogUDPClient) Format(format encoders.Format) *SyslogUDPClient {
	c.format = format
	return c
}

func (c *SyslogUDPClient) Close() (err error) {
	c.Lock()
	defer c.Unlock()
	if c.conn == nil {
		return nil
	}
	err = c.conn.Close()
	c.conn = nil
	return err
}

func (c *SyslogUDPClient) Connect() (err error) {
	c.Lock()
	defer func() {
		if err != nil {
			c.conn = nil
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
			return fmt.Errorf("SyslogUDPClient: specify a host or a unix path")
		}
		if c.port == 0 {
			return fmt.Errorf("SyslogUDPClient: specify a port")
		}
		hostport := net.JoinHostPort(c.host, strconv.FormatInt(int64(c.port), 10))
		conn, err = net.Dial("udp", hostport)
		if err != nil {
			return err
		}
		udpconn := conn.(*net.UDPConn)
		udpconn.SetWriteBuffer(65536)
		c.conn = udpconn
	} else {
		conn, err = net.Dial("unixgram", c.path)
		if err != nil {
			return err
		}
		unixconn := conn.(*net.UnixConn)
		unixconn.SetWriteBuffer(65536)
		c.conn = unixconn
	}
	return nil
}

func (c *SyslogUDPClient) Flush() error {
	return nil
}

func (c *SyslogUDPClient) Ack() chan ulid.ULID {
	return nil
}

func (c *SyslogUDPClient) Nack() chan ulid.ULID {
	return nil
}

func (c *SyslogUDPClient) Send(msg *model.FullMessage) (err error) {
	// may be called concurrently
	if c.conn == nil {
		return fmt.Errorf("SyslogUDPClient: not connected")
	}
	if msg == nil {
		return nil
	}
	buf, err := encoders.ChainEncode(c.encoder, msg)
	if err != nil {
		return encoders.NonEncodableError
	}
	if len(buf) == 0 {
		return nil
	}
	_, err = c.conn.Write(buf)
	return err
}

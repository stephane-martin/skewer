package binder

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/utils"
)

// TODO: encrypt streams

func IsStream(lnet string) bool {
	switch lnet {
	case "tcp", "tcp4", "tcp6", "unix", "unixpacket":
		return true
	default:
		return false
	}
}

type FileConn struct {
	net.Conn
	uid string
	err string
}

func (c *FileConn) Close() error {
	return c.Conn.Close()
}

type FilePacketConn struct {
	net.PacketConn
	uid string
	err string
}

func (c *FilePacketConn) Close() error {
	return c.PacketConn.Close()
}

type ExternalConns struct {
	conns map[string](chan *FileConn)
	sync.Mutex
}

func NewExternalConns() *ExternalConns {
	ext := ExternalConns{conns: map[string](chan *FileConn){}}
	return &ext
}

func (c *ExternalConns) Close() {
	c.Lock()
	for _, conn := range c.conns {
		close(conn)
	}
	c.Unlock()
}

func (c *ExternalConns) Push(addr string, conn *FileConn) {
	c.Lock()
	if i, ok := c.conns[addr]; ok && i != nil {
		i <- conn
	}
	c.Unlock()
}

func (c *ExternalConns) Get(addr string, create bool) chan *FileConn {
	c.Lock()
	defer c.Unlock()
	if i, ok := c.conns[addr]; ok && i != nil {
		return i
	}
	if create {
		c.conns[addr] = make(chan *FileConn)
		return c.conns[addr]
	}
	return nil
}

func (c *ExternalConns) Delete(addr string) {
	c.Lock()
	conn, ok := c.conns[addr]
	if ok {
		delete(c.conns, addr)
		if conn != nil {
			close(conn)
		}
	}
	c.Unlock()
}

type ExternalPacketConns struct {
	conns map[string](chan *FilePacketConn)
	sync.Mutex
}

func (c *ExternalPacketConns) Close() {
	c.Lock()
	for _, conn := range c.conns {
		close(conn)
	}
	c.Unlock()
}

func (c *ExternalPacketConns) Push(addr string, conn *FilePacketConn) {
	c.Lock()
	if i, ok := c.conns[addr]; ok && i != nil {
		i <- conn
	}
	c.Unlock()
}

func (c *ExternalPacketConns) Get(addr string, create bool) chan *FilePacketConn {
	c.Lock()
	defer c.Unlock()
	if i, ok := c.conns[addr]; ok && i != nil {
		return i
	}
	if create {
		c.conns[addr] = make(chan *FilePacketConn)
		return c.conns[addr]
	}
	return nil
}

func (c *ExternalPacketConns) Delete(addr string) {
	c.Lock()
	conn, ok := c.conns[addr]
	if ok {
		delete(c.conns, addr)
		if conn != nil {
			close(conn)
		}
	}
	c.Unlock()
}

func NewExternalPacketConns() *ExternalPacketConns {
	ext := ExternalPacketConns{conns: map[string](chan *FilePacketConn){}}
	return &ext
}

type BinderClient struct {
	conn           *net.UnixConn
	NewStreamConns *ExternalConns
	NewPacketConns *ExternalPacketConns
}

func NewBinderClient(binderFile *os.File, logger log15.Logger) (*BinderClient, error) {
	conn, err := net.FileConn(binderFile)
	if err != nil {
		return nil, err
	}
	binderFile.Close()
	c := BinderClient{conn: conn.(*net.UnixConn)}
	c.NewStreamConns = NewExternalConns()
	c.NewPacketConns = NewExternalPacketConns()

	go func() {
		for {
			buf := make([]byte, 1024)
			oob := make([]byte, syscall.CmsgSpace(4))
			n, oobn, _, _, err := c.conn.ReadMsgUnix(buf, oob)
			if err != nil {
				logger.Debug("Error reading message from parent binder", "error", err)
				c.NewStreamConns.Close()
				c.NewPacketConns.Close()
				return
			}
			if n > 0 {
				msg := strings.Trim(string(buf[:n]), " \r\n")
				logger.Debug("received message from root parent", "message", msg)
				if strings.HasPrefix(msg, "error ") {
					parts := strings.SplitN(msg, " ", 3)
					addr := parts[1]
					lnet := strings.SplitN(addr, ":", 2)[0]
					errorstr := parts[2]

					if IsStream(lnet) {
						c.NewStreamConns.Push(addr, &FileConn{err: errorstr})
					} else {
						c.NewPacketConns.Push(addr, &FilePacketConn{err: errorstr})
					}
				}

				if strings.HasPrefix(msg, "confirmlisten ") {
					parts := strings.SplitN(msg, " ", 2)
					addr := parts[1]
					c.NewStreamConns.Push(addr, &FileConn{})
				}

				if strings.HasPrefix(msg, "newconn ") && oobn > 0 {
					parts := strings.SplitN(msg, " ", 3)
					uid := parts[1]
					addr := parts[2]
					cmsgs, err := syscall.ParseSocketControlMessage(oob)
					if err == nil {
						fds, err := syscall.ParseUnixRights(&cmsgs[0])
						if err == nil {
							for _, fd := range fds {
								logger.Debug("Received a new connection from root parent", "uid", uid, "addr", addr)
								lnet := strings.SplitN(addr, ":", 2)[0]
								rf := os.NewFile(uintptr(fd), "newconn")
								if IsStream(lnet) {
									rc, err := net.FileConn(rf)
									rf.Close()
									if err == nil {
										c.NewStreamConns.Push(addr, &FileConn{Conn: rc, uid: uid})
									}
								} else {
									rc, err := net.FilePacketConn(rf)
									rf.Close()
									if err == nil {
										c.NewPacketConns.Push(addr, &FilePacketConn{PacketConn: rc, uid: uid})
									}
								}
							}
						} else {
							logger.Warn("ParseUnixRights() error", "error", err)
						}
					} else {
						logger.Warn("ParseSocketControlMessage() error", "error", err)
					}
				}
			}
		}
	}()

	return &c, nil
}

type BinderListener struct {
	addr   string
	client *BinderClient
}

func (l *BinderListener) Close() error {
	l.client.StopListen(l.addr)
	return nil
}

func (l *BinderListener) Accept() (net.Conn, error) {
	ichan := l.client.NewStreamConns.Get(l.addr, false)
	notListenErr := &net.OpError{Err: fmt.Errorf("Not listening on that address"), Addr: l.Addr(), Op: "Accept"}
	if ichan == nil {
		return nil, notListenErr
	}
	conn, more := <-ichan
	if !more {
		return nil, notListenErr
	}
	if len(conn.err) > 0 {
		return nil, &net.OpError{Err: fmt.Errorf(conn.err), Addr: l.Addr(), Op: "Accept"}
	}
	return conn, nil
}

type BinderAddr struct {
	addr string
}

func (a *BinderAddr) String() string {
	parts := strings.SplitN(a.addr, ":", 2)
	if len(parts) == 2 {
		return parts[1]
	} else {
		return ""
	}
}

func (a *BinderAddr) Network() string {
	return strings.SplitN(a.addr, ":", 2)[0]
}

func (l *BinderListener) Addr() net.Addr {
	return &BinderAddr{addr: l.addr}
}

func (c *BinderClient) Listen(lnet string, laddr string) (net.Listener, error) {
	addr := fmt.Sprintf("%s:%s", lnet, laddr)
	ichan := c.NewStreamConns.Get(addr, true)
	_, err := c.conn.Write([]byte(fmt.Sprintf("listen %s\n", addr)))
	if err != nil {
		return nil, err
	}

	confirmation, more := <-ichan
	if !more {
		return nil, &net.OpError{Err: fmt.Errorf("closed ichan"), Op: "Listen"}
	} else if len(confirmation.err) > 0 {
		return nil, &net.OpError{Err: fmt.Errorf(confirmation.err), Op: "Listen"}
	} else {
		return &BinderListener{addr: addr, client: c}, nil
	}
}

func (c *BinderClient) ListenPacket(lnet string, laddr string) (net.PacketConn, error) {
	addr := fmt.Sprintf("%s:%s", lnet, laddr)
	ichan := c.NewPacketConns.Get(addr, true)
	_, err := c.conn.Write([]byte(fmt.Sprintf("listen %s\n", addr)))
	if err != nil {
		return nil, err
	}
	conn, more := <-ichan
	c.NewPacketConns.Delete(addr)
	if !more {
		return nil, fmt.Errorf("There was an error when asking the parent process for a packet connection")
	}
	if len(conn.err) > 0 {
		return nil, fmt.Errorf(conn.err)
	}
	return conn, nil
}

func (c *BinderClient) StopListen(addr string) {
	c.NewStreamConns.Delete(addr)
	_, _ = c.conn.Write([]byte(fmt.Sprintf("stoplisten %s\n", addr)))
}

func (c *BinderClient) Quit() error {
	return utils.All(
		func() (err error) {
			_, err = c.conn.Write([]byte("byebye\n"))
			return err
		},
		func() error {
			return c.conn.Close()
		},
	)
}

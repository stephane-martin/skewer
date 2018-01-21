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
	defer c.Unlock()
	for _, conn := range c.conns {
		close(conn)
	}
	c.conns = map[string](chan *FileConn){}
}

func (c *ExternalConns) Push(addr string, conn *FileConn) {
	c.Lock()
	defer c.Unlock()
	if i, ok := c.conns[addr]; ok && i != nil {
		i <- conn
	} else {
		fmt.Fprintln(os.Stderr, "Push failed: chan does not exist")
	}
}

func (c *ExternalConns) Get(addr string, create bool) chan *FileConn {
	c.Lock()
	defer c.Unlock()
	if i, ok := c.conns[addr]; ok && i != nil {
		return i
	}
	if create {
		c.conns[addr] = make(chan *FileConn, 16)
		return c.conns[addr]
	}
	return nil
}

func (c *ExternalConns) Delete(addr string) bool {
	c.Lock()
	defer c.Unlock()
	conn, ok := c.conns[addr]
	if ok {
		delete(c.conns, addr)
		if conn != nil {
			close(conn)
		}
		return true
	}
	return false
}

type ExternalPacketConns struct {
	conns map[string](chan *FilePacketConn)
	sync.Mutex
}

func (c *ExternalPacketConns) Close() {
	c.Lock()
	defer c.Unlock()
	for _, conn := range c.conns {
		close(conn)
	}
	c.conns = map[string](chan *FilePacketConn){}
}

func (c *ExternalPacketConns) Push(addr string, conn *FilePacketConn) {
	c.Lock()
	defer c.Unlock()
	if i, ok := c.conns[addr]; ok && i != nil {
		i <- conn
	} else {
		fmt.Fprintln(os.Stderr, "Push failed: chan does not exist")
	}
}

func (c *ExternalPacketConns) Get(addr string, create bool) chan *FilePacketConn {
	c.Lock()
	defer c.Unlock()
	if i, ok := c.conns[addr]; ok && i != nil {
		return i
	}
	if create {
		c.conns[addr] = make(chan *FilePacketConn, 16)
		return c.conns[addr]
	}
	return nil
}

func (c *ExternalPacketConns) Delete(addr string) bool {
	c.Lock()
	defer c.Unlock()
	conn, ok := c.conns[addr]
	if ok {
		delete(c.conns, addr)
		if conn != nil {
			close(conn)
		}
		return true
	}
	return false
}

func NewExternalPacketConns() *ExternalPacketConns {
	ext := ExternalPacketConns{conns: map[string](chan *FilePacketConn){}}
	return &ext
}

type BinderClientImpl struct {
	conn           *net.UnixConn
	NewStreamConns *ExternalConns
	NewPacketConns *ExternalPacketConns
}

func NewBinderClient(binderFile *os.File, logger log15.Logger) (*BinderClientImpl, error) {
	conn, err := net.FileConn(binderFile)
	if err != nil {
		return nil, err
	}
	binderFile.Close()
	c := BinderClientImpl{conn: conn.(*net.UnixConn)}
	c.NewStreamConns = NewExternalConns()
	c.NewPacketConns = NewExternalPacketConns()

	go func() {
		for {
			buf := make([]byte, 1024)
			oob := make([]byte, syscall.CmsgSpace(4))
			n, oobn, _, _, err := c.conn.ReadMsgUnix(buf, oob)
			if err != nil {
				logger.Error("Error reading message from parent binder", "error", err)
				c.NewStreamConns.Close()
				c.NewPacketConns.Close()
				// the skewer parent process may be gone... for whatever reason
				// we should stop before catastrophe happens
				pid := os.Getpid()
				process, err := os.FindProcess(pid)
				if err == nil {
					process.Signal(syscall.SIGTERM)
				}
				return
			}
			if n > 0 {
				msgs := strings.Split(strings.Trim(string(buf[:n]), " \r\n"), "\n")
				logger.Debug("received messages from root parent", "messages", msgs)
				for _, msg := range msgs {
					msg = strings.Trim(msg, " \r\n")
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
										} else {
											logger.Warn("Error getting connection from file handler", "error", err)
										}
									} else {
										rc, err := net.FilePacketConn(rf)
										rf.Close()
										if err == nil {
											c.NewPacketConns.Push(addr, &FilePacketConn{PacketConn: rc, uid: uid})
										} else {
											logger.Warn("Error getting connection from file handler", "error", err)
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
		}
	}()

	return &c, nil
}

type BinderListener struct {
	addr   string
	client *BinderClientImpl
}

func (l *BinderListener) Close() error {
	return l.client.StopListen(l.addr)
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

func (c *BinderClientImpl) Listen(lnet string, laddr string) (net.Listener, error) {
	addr := fmt.Sprintf("%s:%s", lnet, laddr)
	ichan := c.NewStreamConns.Get(addr, true)
	// TODO: generate UID in the client instead of server
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

func (c *BinderClientImpl) ListenPacket(lnet string, laddr string) (net.PacketConn, error) {
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

func (c *BinderClientImpl) StopListen(addr string) error {
	ok := c.NewStreamConns.Delete(addr)
	if ok {
		_, _ = c.conn.Write([]byte(fmt.Sprintf("stoplisten %s\n", addr)))
		return nil
	}
	return fmt.Errorf("Already closed")
}

func (c *BinderClientImpl) Quit() error {
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

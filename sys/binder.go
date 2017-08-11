package sys

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"

	"github.com/inconshreveable/log15"
)

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
	f          *os.File
	uid        string
	parentConn *net.UnixConn
	err        string
	confirmed  bool
}

func (c *FileConn) Close() error {
	err := c.Conn.Close()
	c.f.Close()
	c.parentConn.Write([]byte(fmt.Sprintf("closeconn %s\n", c.uid)))
	return err
}

type FilePacketConn struct {
	net.PacketConn
	f          *os.File
	uid        string
	parentConn *net.UnixConn
	err        string
}

func (c *FilePacketConn) Close() error {
	err := c.PacketConn.Close()
	c.f.Close()
	c.parentConn.Write([]byte(fmt.Sprintf("closeconn %s\n", c.uid)))
	return err
}

type BinderClient struct {
	childFile          *os.File
	parentConn         *net.UnixConn
	IncomingConn       map[string]chan *FileConn
	IncomingPacketConn map[string]chan *FilePacketConn
	iconnMu            *sync.Mutex
	ipacketMu          *sync.Mutex
}

func NewBinderClient(binderFile *os.File, logger log15.Logger) (*BinderClient, error) {
	genconn, err := net.FileConn(binderFile)
	if err != nil {
		return nil, err
	}
	conn := genconn.(*net.UnixConn)
	c := BinderClient{childFile: binderFile, parentConn: conn}
	c.IncomingConn = map[string]chan *FileConn{}
	c.IncomingPacketConn = map[string]chan *FilePacketConn{}
	c.iconnMu = &sync.Mutex{}
	c.ipacketMu = &sync.Mutex{}

	go func() {
		for {
			buf := make([]byte, 1024)
			oob := make([]byte, syscall.CmsgSpace(4))
			n, oobn, _, _, err := c.parentConn.ReadMsgUnix(buf, oob)
			if err != nil {
				logger.Error("Error reading message from parent binder", "error", err)
				for _, ichan := range c.IncomingConn {
					close(ichan)
				}

				for _, ichan := range c.IncomingPacketConn {
					close(ichan)
				}
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
						c.iconnMu.Lock()
						if i, ok := c.IncomingConn[addr]; ok {
							if i != nil {
								i <- &FileConn{err: errorstr, parentConn: c.parentConn, confirmed: false}
							}
						}
						c.iconnMu.Unlock()
					} else {
						c.ipacketMu.Lock()
						if i, ok := c.IncomingPacketConn[addr]; ok {
							if i != nil {
								i <- &FilePacketConn{err: errorstr, parentConn: c.parentConn}
								close(i)
							}
						}
						c.ipacketMu.Unlock()
					}
				}

				if strings.HasPrefix(msg, "confirmlisten ") {
					parts := strings.SplitN(msg, " ", 2)
					addr := parts[1]
					c.iconnMu.Lock()
					if i, ok := c.IncomingConn[addr]; ok {
						if i != nil {
							i <- &FileConn{parentConn: c.parentConn, confirmed: true}
						}
					}
					c.iconnMu.Unlock()
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
									if err == nil {
										c.iconnMu.Lock()
										if i, ok := c.IncomingConn[addr]; ok {
											if i != nil {
												i <- &FileConn{Conn: rc, f: rf, uid: uid, parentConn: c.parentConn}
											}
										}
										c.iconnMu.Unlock()
									}
								} else {
									rc, err := net.FilePacketConn(rf)
									if err == nil {
										c.ipacketMu.Lock()
										if i, ok := c.IncomingPacketConn[addr]; ok {
											if i != nil {
												i <- &FilePacketConn{PacketConn: rc, f: rf, uid: uid, parentConn: c.parentConn}
												close(i)
											}
										}
										c.ipacketMu.Unlock()
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
	l.client.iconnMu.Lock()
	ichan, ok := l.client.IncomingConn[l.addr]
	l.client.iconnMu.Unlock()
	notListenErr := &net.OpError{Err: fmt.Errorf("Not listening on that address"), Addr: l.Addr(), Op: "Accept"}
	if !ok {
		return nil, notListenErr
	}
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
	c.iconnMu.Lock()
	ichan, ok := c.IncomingConn[addr]
	if !ok {
		c.IncomingConn[addr] = make(chan *FileConn)
		ichan = c.IncomingConn[addr]
	}
	c.iconnMu.Unlock()
	c.parentConn.Write([]byte(fmt.Sprintf("listen %s\n", addr)))

	confirmation, more := <-ichan
	if !more {
		return nil, &net.OpError{Err: fmt.Errorf("Closed ichan?!"), Op: "Listen"}
	} else if len(confirmation.err) > 0 {
		return nil, &net.OpError{Err: fmt.Errorf(confirmation.err), Op: "Listen"}
	} else {
		return &BinderListener{addr: addr, client: c}, nil
	}
}

func (c *BinderClient) ListenPacket(lnet string, laddr string) (net.PacketConn, error) {
	addr := fmt.Sprintf("%s:%s", lnet, laddr)
	c.ipacketMu.Lock()
	ichan, ok := c.IncomingPacketConn[addr]
	if !ok {
		c.IncomingPacketConn[addr] = make(chan *FilePacketConn)
		ichan = c.IncomingPacketConn[addr]
	}
	c.ipacketMu.Unlock()
	c.parentConn.Write([]byte(fmt.Sprintf("listen %s\n", addr)))
	conn, more := <-ichan
	c.ipacketMu.Lock()
	delete(c.IncomingPacketConn, addr)
	c.ipacketMu.Unlock()
	if !more {
		return nil, nil // should not happen
	}
	if len(conn.err) > 0 {
		return nil, fmt.Errorf(conn.err)
	}
	return conn, nil
}

func (c *BinderClient) StopListen(addr string) {
	c.iconnMu.Lock()
	c.parentConn.Write([]byte(fmt.Sprintf("stoplisten %s\n", addr)))
	ichan, ok := c.IncomingConn[addr]
	delete(c.IncomingConn, addr)
	if ok && ichan != nil {
		close(ichan)
	}
	c.iconnMu.Unlock()
}

func (c *BinderClient) Quit() {
	c.parentConn.Write([]byte("byebye\n"))
	c.parentConn.Close()
	c.childFile.Close()
}

func (c *BinderClient) Reset() {
	c.parentConn.Write([]byte("reset\n"))
}

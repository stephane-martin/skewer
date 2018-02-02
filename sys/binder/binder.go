package binder

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"

	"github.com/awnumar/memguard"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/utils"
)

func IsStream(lnet string) bool {
	switch lnet {
	case "tcp", "tcp4", "tcp6", "unix", "unixpacket":
		return true
	default:
		return false
	}
}

type fileConn struct {
	net.Conn
	uid string
	err string
}

func (c *fileConn) Close() error {
	return c.Conn.Close()
}

type filePConn struct {
	net.PacketConn
	uid string
	err string
}

func (c *filePConn) SetWriteBuffer(bytes int) error {
	if uc, ok := c.PacketConn.(*net.UnixConn); ok {
		return uc.SetWriteBuffer(bytes)
	}
	if uc, ok := c.PacketConn.(*net.UDPConn); ok {
		return uc.SetWriteBuffer(bytes)
	}
	return nil
}

func (c *filePConn) SetReadBuffer(bytes int) error {
	if uc, ok := c.PacketConn.(*net.UnixConn); ok {
		return uc.SetReadBuffer(bytes)
	}
	if uc, ok := c.PacketConn.(*net.UDPConn); ok {
		return uc.SetReadBuffer(bytes)
	}
	return nil
}

func (c *filePConn) Close() error {
	return c.PacketConn.Close()
}

type extConns struct {
	conns map[string](chan *fileConn)
	sync.Mutex
}

func newExtConns() *extConns {
	ext := extConns{conns: map[string](chan *fileConn){}}
	return &ext
}

func (c *extConns) close() {
	c.Lock()
	defer c.Unlock()
	for _, conn := range c.conns {
		close(conn)
	}
	c.conns = map[string](chan *fileConn){}
}

func (c *extConns) push(addr string, conn *fileConn) {
	c.Lock()
	defer c.Unlock()
	if i, ok := c.conns[addr]; ok && i != nil {
		i <- conn
	} else {
		fmt.Fprintln(os.Stderr, "Push failed: chan does not exist")
	}
}

func (c *extConns) get(addr string, create bool) chan *fileConn {
	c.Lock()
	defer c.Unlock()
	if i, ok := c.conns[addr]; ok && i != nil {
		return i
	}
	if create {
		c.conns[addr] = make(chan *fileConn, 16)
		return c.conns[addr]
	}
	return nil
}

func (c *extConns) delete(addr string) bool {
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

type extPConns struct {
	conns map[string](chan *filePConn)
	sync.Mutex
}

func (c *extPConns) close() {
	c.Lock()
	defer c.Unlock()
	for _, conn := range c.conns {
		close(conn)
	}
	c.conns = map[string](chan *filePConn){}
}

func (c *extPConns) push(addr string, conn *filePConn) {
	c.Lock()
	defer c.Unlock()
	if i, ok := c.conns[addr]; ok && i != nil {
		i <- conn
	} else {
		fmt.Fprintln(os.Stderr, "Push failed: chan does not exist")
	}
}

func (c *extPConns) get(addr string, create bool) chan *filePConn {
	c.Lock()
	defer c.Unlock()
	if i, ok := c.conns[addr]; ok && i != nil {
		return i
	}
	if create {
		c.conns[addr] = make(chan *filePConn, 16)
		return c.conns[addr]
	}
	return nil
}

func (c *extPConns) delete(addr string) bool {
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

func newExtPConns() *extPConns {
	ext := extPConns{conns: map[string](chan *filePConn){}}
	return &ext
}

type clientImpl struct {
	conn      *net.UnixConn
	newConns  *extConns
	newPConns *extPConns
	writer    *utils.EncryptWriter
}

type scannerOob struct {
	conn   *net.UnixConn
	buf    []byte
	oobbuf []byte
	split  bufio.SplitFunc
}

func newScannerOob(conn *net.UnixConn, secret *memguard.LockedBuffer) *scannerOob {
	return &scannerOob{
		conn:   conn,
		buf:    make([]byte, 0, 2048),
		oobbuf: make([]byte, 0, 2048),
		split:  utils.MakeDecryptSplit(secret),
	}
}

var oobSize = syscall.CmsgSpace(4)

func (s *scannerOob) Read() (msg string, oob []byte, err error) {
	var adv int
	var token []byte

	for {
		adv, token, err = s.split(s.buf, false)
		if err != nil {
			return "", nil, err
		}
		if adv != 0 {
			break
		}
		buf := make([]byte, 2048)
		oobbuf := make([]byte, 2048)
		n, oobn, _, _, err := s.conn.ReadMsgUnix(buf, oobbuf)
		if err != nil {
			return "", nil, err
		}
		if n > 0 {
			s.buf = append(s.buf, buf[:n]...)
		}
		if oobn > 0 {
			s.oobbuf = append(s.oobbuf, oobbuf[:oobn]...)
		}
	}

	s.buf = s.buf[adv:]
	msg = string(token)
	if strings.HasPrefix(msg, "newconn ") {
		if len(s.oobbuf) < oobSize {
			return "", nil, fmt.Errorf("Not enough out of band data: %d < %d", len(s.oobbuf), oobSize)
		}
		oob = s.oobbuf[:oobSize]
		s.oobbuf = s.oobbuf[oobSize:]
	}
	return msg, oob, nil
}

func NewClient(binderFile *os.File, secret *memguard.LockedBuffer, logger log15.Logger) (Client, error) {
	conn, err := net.FileConn(binderFile)
	if err != nil {
		return nil, err
	}
	binderFile.Close()
	c := clientImpl{
		conn:   conn.(*net.UnixConn),
		writer: utils.NewEncryptWriter(conn, secret),
	}
	c.newConns = newExtConns()
	c.newPConns = newExtPConns()
	scanner := newScannerOob(c.conn, secret)

	go func() {
		for {
			msg, oob, err := scanner.Read()
			if err != nil {
				logger.Error("Error reading message from parent binder", "error", err)
				c.newConns.close()
				c.newPConns.close()
				// the skewer parent process may be gone... for whatever reason
				// we should stop before some catastrophe happens
				pid := os.Getpid()
				process, err := os.FindProcess(pid)
				if err == nil {
					process.Signal(syscall.SIGTERM)
				}
				return
			}
			if len(msg) > 0 {

				logger.Debug("received message from root parent", "message", msg)
				if strings.HasPrefix(msg, "error ") {
					parts := strings.SplitN(msg, " ", 3)
					addr := parts[1]
					lnet := strings.SplitN(addr, ":", 2)[0]
					errorstr := parts[2]

					if IsStream(lnet) {
						c.newConns.push(addr, &fileConn{err: errorstr})
					} else {
						c.newPConns.push(addr, &filePConn{err: errorstr})
					}
				}

				if strings.HasPrefix(msg, "confirmlisten ") {
					parts := strings.SplitN(msg, " ", 2)
					addr := parts[1]
					c.newConns.push(addr, &fileConn{})
				}

				if strings.HasPrefix(msg, "newconn ") && len(oob) > 0 {
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
										c.newConns.push(addr, &fileConn{Conn: rc, uid: uid})
									} else {
										logger.Warn("Error getting connection from file handler", "error", err)
									}
								} else {
									rc, err := net.FilePacketConn(rf)
									rf.Close()
									if err == nil {
										c.newPConns.push(addr, &filePConn{PacketConn: rc, uid: uid})
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
	}()

	return &c, nil
}

type Listener struct {
	addr   string
	client *clientImpl
}

func (l *Listener) Close() error {
	return l.client.StopListen(l.addr)
}

func (l *Listener) Accept() (net.Conn, error) {
	ichan := l.client.newConns.get(l.addr, false)
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

type addrType struct {
	addr string
}

func (a *addrType) String() string {
	parts := strings.SplitN(a.addr, ":", 2)
	if len(parts) == 2 {
		return parts[1]
	} else {
		return ""
	}
}

func (a *addrType) Network() string {
	return strings.SplitN(a.addr, ":", 2)[0]
}

func (l *Listener) Addr() net.Addr {
	return &addrType{addr: l.addr}
}

func (c *clientImpl) Listen(lnet string, laddr string) (net.Listener, error) {
	addr := fmt.Sprintf("%s:%s", lnet, laddr)
	ichan := c.newConns.get(addr, true)
	// TODO: generate UID in the client instead of server
	_, err := c.writer.Write([]byte(fmt.Sprintf("listen %s", addr)))
	if err != nil {
		return nil, err
	}

	confirmation, more := <-ichan
	if !more {
		return nil, &net.OpError{Err: fmt.Errorf("closed ichan"), Op: "Listen"}
	} else if len(confirmation.err) > 0 {
		return nil, &net.OpError{Err: fmt.Errorf(confirmation.err), Op: "Listen"}
	} else {
		return &Listener{addr: addr, client: c}, nil
	}
}

func (c *clientImpl) ListenPacket(lnet string, laddr string, bytes int) (net.PacketConn, error) {
	addr := fmt.Sprintf("%s:%s", lnet, laddr)
	ichan := c.newPConns.get(addr, true)
	_, err := c.writer.Write([]byte(fmt.Sprintf("listen %s", addr)))
	if err != nil {
		return nil, err
	}
	conn, more := <-ichan
	c.newPConns.delete(addr)
	if !more {
		return nil, fmt.Errorf("There was an error when asking the parent process for a packet connection")
	}
	if len(conn.err) > 0 {
		return nil, fmt.Errorf(conn.err)
	}
	if bytes > 0 {
		conn.SetReadBuffer(bytes)
		conn.SetWriteBuffer(bytes)
	}
	return conn, nil
}

func (c *clientImpl) StopListen(addr string) error {
	ok := c.newConns.delete(addr)
	if ok {
		_, _ = c.writer.Write([]byte(fmt.Sprintf("stoplisten %s", addr)))
		return nil
	}
	return fmt.Errorf("Already closed")
}

func (c *clientImpl) Quit() error {
	return utils.All(
		func() (err error) {
			_, err = c.writer.Write([]byte("byebye"))
			return err
		},
		func() error {
			return c.conn.Close()
		},
	)
}

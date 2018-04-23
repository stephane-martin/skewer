package binder

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/awnumar/memguard"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
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
	err error
}

func (c *fileConn) SetWriteBuffer(bytes int) error {
	if uc, ok := c.Conn.(*net.UnixConn); ok {
		return uc.SetWriteBuffer(bytes)
	}
	if uc, ok := c.Conn.(*net.TCPConn); ok {
		return uc.SetWriteBuffer(bytes)
	}
	return nil
}

func (c *fileConn) SetReadBuffer(bytes int) error {
	if uc, ok := c.Conn.(*net.UnixConn); ok {
		return uc.SetReadBuffer(bytes)
	}
	if uc, ok := c.Conn.(*net.TCPConn); ok {
		return uc.SetReadBuffer(bytes)
	}
	return nil
}

func (c *fileConn) Close() error {
	return c.Conn.Close()
}

type filePConn struct {
	net.PacketConn
	uid string
	err error
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
	return &extConns{conns: map[string](chan *fileConn){}}
}

func (c *extConns) close() {
	c.Lock()
	for _, conn := range c.conns {
		close(conn)
	}
	c.conns = map[string](chan *fileConn){}
	c.Unlock()
}

func (c *extConns) push(addr string, conn *fileConn) {
	c.Lock()
	if i, ok := c.conns[addr]; ok && i != nil {
		i <- conn
	} else {
		fmt.Fprintln(os.Stderr, "Push failed: chan does not exist")
	}
	c.Unlock()
}

func (c *extConns) get(addr string, create bool) (conn chan *fileConn) {
	var ok bool
	c.Lock()
	if conn, ok = c.conns[addr]; (!ok || conn == nil) && create {
		c.conns[addr] = make(chan *fileConn, 16)
		conn = c.conns[addr]
	}
	c.Unlock()
	return conn
}

func (c *extConns) delete(addr string) (ok bool) {
	var conn chan *fileConn
	c.Lock()
	if conn, ok = c.conns[addr]; ok {
		delete(c.conns, addr)
		if conn != nil {
			close(conn)
		}
	}
	c.Unlock()
	return ok
}

type extPConns struct {
	conns map[string](chan *filePConn)
	sync.Mutex
}

func (c *extPConns) close() {
	c.Lock()
	for _, conn := range c.conns {
		close(conn)
	}
	c.conns = map[string](chan *filePConn){}
	c.Unlock()
}

func (c *extPConns) push(addr string, conn *filePConn) {
	c.Lock()
	if i, ok := c.conns[addr]; ok && i != nil {
		i <- conn
	} else {
		fmt.Fprintln(os.Stderr, "Push failed: chan does not exist")
	}
	c.Unlock()
}

func (c *extPConns) get(addr string, create bool) (conn chan *filePConn) {
	var ok bool
	c.Lock()
	if conn, ok = c.conns[addr]; (!ok || conn == nil) && create {
		c.conns[addr] = make(chan *filePConn, 16)
		conn = c.conns[addr]
	}
	c.Unlock()
	return conn
}

func (c *extPConns) delete(addr string) (ok bool) {
	var conn chan *filePConn
	c.Lock()
	if conn, ok = c.conns[addr]; ok {
		delete(c.conns, addr)
		if conn != nil {
			close(conn)
		}
	}
	c.Unlock()
	return ok
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
	logger    log15.Logger
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
			return "", nil, eerrors.Errorf("Not enough out of band data: %d < %d", len(s.oobbuf), oobSize)
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
		logger: logger,
	}
	c.newConns = newExtConns()
	c.newPConns = newExtPConns()
	scanner := newScannerOob(c.conn, secret)

	go func() {
		defer func() {
			if e := eerrors.Err(recover()); e != nil {
				logger.Crit("panic in binder client", "error", e.Error())
				// the skewer parent process may be gone... for whatever reason
				// we should stop before some catastrophe happens
				c.newConns.close()
				c.newPConns.close()
				pid := os.Getpid()
				process, err := os.FindProcess(pid)
				if err == nil {
					process.Signal(syscall.SIGTERM)
				}
			}
		}()
		for {
			msg, oob, err := scanner.Read()
			if err != nil {
				panic(fmt.Sprintf("Error reading message from parent binder: %s", err))
			}
			if len(msg) > 0 {

				logger.Debug("received message from root parent", "message", msg)
				if strings.HasPrefix(msg, "error ") {
					parts := strings.SplitN(msg, " ", 3)
					addr := parts[1]
					lnet := strings.SplitN(addr, ":", 2)[0]
					err := eerrors.New(parts[2])

					if IsStream(lnet) {
						c.newConns.push(addr, &fileConn{err: err})
					} else {
						c.newPConns.push(addr, &filePConn{err: err})
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
	addr      string
	client    *clientImpl
	keepalive bool
	period    time.Duration
	logger    log15.Logger
}

func (l *Listener) Close() error {
	return l.client.StopListen(l.addr)
}

func (l *Listener) Accept() (net.Conn, error) {
	ichan := l.client.newConns.get(l.addr, false)
	notListenErr := &net.OpError{Err: errors.New("Not listening on that address"), Addr: l.Addr(), Op: "Accept"}
	if ichan == nil {
		return nil, notListenErr
	}
	conn, more := <-ichan
	if !more {
		return nil, notListenErr
	}
	if conn.err != nil {
		return nil, &net.OpError{Err: conn.err, Addr: l.Addr(), Op: "Accept"}
	}
	if c, ok := conn.Conn.(*net.TCPConn); ok {
		if l.keepalive {
			err := c.SetKeepAlive(true)
			if err != nil {
				l.logger.Warn("Error setting keepalive on accepted connection", "error", err)
			} else {
				err := c.SetKeepAlivePeriod(l.period)
				if err != nil {
					l.logger.Warn("Error setting keepalive period on accepted connection", "error", err)
				}
			}
		}
		err := c.SetNoDelay(true)
		if err != nil {
			l.logger.Warn("Error setting nodelay on accepted connection", "error", err)
		}
		err = c.SetLinger(-1)
		if err != nil {
			l.logger.Warn("Error setting linger on accepted connection", "error", err)
		}
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
	}
	return ""
}

func (a *addrType) Network() string {
	return strings.SplitN(a.addr, ":", 2)[0]
}

func (l *Listener) Addr() net.Addr {
	return &addrType{addr: l.addr}
}

func (c *clientImpl) Listen(lnet string, laddr string) (net.Listener, error) {
	return c.ListenKeepAlive(lnet, laddr, 0)
}

func (c *clientImpl) ListenKeepAlive(lnet string, laddr string, period time.Duration) (l net.Listener, err error) {
	addr := fmt.Sprintf("%s:%s", lnet, laddr)
	ichan := c.newConns.get(addr, true)
	_, err = c.writer.Write([]byte(fmt.Sprintf("listen %s", addr)))
	if err != nil {
		return nil, err
	}

	confirmation, more := <-ichan
	if !more {
		return nil, &net.OpError{Err: errors.New("closed ichan"), Op: "Listen"}
	} else if confirmation.err != nil {
		return nil, &net.OpError{Err: confirmation.err, Op: "Listen"}
	}
	if period != 0 {
		l = &Listener{addr: addr, client: c, keepalive: true, period: period, logger: c.logger}
	} else {
		l = &Listener{addr: addr, client: c, logger: c.logger}
	}
	return l, nil
}

func (c *clientImpl) ListenPacket(lnet string, laddr string, bytes int) (pconn net.PacketConn, err error) {
	var more bool
	var conn *filePConn

	addr := fmt.Sprintf("%s:%s", lnet, laddr)
	ichan := c.newPConns.get(addr, true)
	_, err = c.writer.Write([]byte(fmt.Sprintf("listen %s", addr)))
	if err != nil {
		return nil, err
	}
	conn, more = <-ichan
	c.newPConns.delete(addr)
	if !more {
		return nil, errors.New("There was an error when asking the parent process for a packet connection")
	}
	if conn.err != nil {
		return nil, conn.err
	}
	pconn = conn
	if bytes > 0 {
		err = conn.SetReadBuffer(bytes)
		if err != nil {
			c.logger.Warn("Error setting read buffer size on packet connection", "error", err)
		}
		err = conn.SetWriteBuffer(bytes)
		if err != nil {
			c.logger.Warn("Error setting write buffer size on packet connection", "error", err)
		}
	}
	return pconn, nil
}

func (c *clientImpl) StopListen(addr string) error {
	if c.newConns.delete(addr) {
		_, _ = c.writer.Write([]byte(fmt.Sprintf("stoplisten %s", addr)))
		return nil
	}
	return errors.New("Already closed")
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

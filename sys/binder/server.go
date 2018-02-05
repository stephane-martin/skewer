package binder

import (
	"bufio"
	"context"
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

type ExternalConn struct {
	Uid  string
	Conn net.Conn
	Addr string
}

type ExternalPacketConn struct {
	Uid  string
	Conn net.PacketConn
	Addr string
}

func listen(ctx context.Context, wg *sync.WaitGroup, logger log15.Logger, schan chan *ExternalConn, addr string) (net.Listener, error) {
	parts := strings.SplitN(addr, ":", 2)
	lnet := parts[0]
	laddr := parts[1]

	l, err := net.Listen(lnet, laddr)

	if err != nil {
		return nil, err
	}

	if lnet == "unix" || lnet == "unixpacket" {
		_ = os.Chmod(laddr, 0777)
		l.(*net.UnixListener).SetUnlinkOnClose(true)
	}

	cctx, cancel := context.WithCancel(ctx)

	wg.Add(1)
	go func() {
		<-cctx.Done()
		_ = l.Close()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			c, err := l.Accept()
			if err == nil {
				uids := utils.NewUidString()
				logger.Debug("New accepted connection", "uid", uids, "addr", addr)
				schan <- &ExternalConn{Uid: uids, Conn: c, Addr: addr}
			} else {
				logger.Debug("Accept error", "error", err, "addr", addr)
				cancel()
				return
			}
		}
	}()

	return l, nil
}

func listenPacket(addr string) (conn net.PacketConn, err error) {
	parts := strings.SplitN(addr, ":", 2)
	lnet := parts[0]
	laddr := parts[1]

	conn, err = net.ListenPacket(lnet, laddr)

	if err != nil {
		return nil, err
	}

	if lnet == "unixgram" {
		_ = os.Chmod(laddr, 0777)
		_ = conn.(*net.UnixConn).SetReadBuffer(65536)
		_ = conn.(*net.UnixConn).SetWriteBuffer(65536)
	} else {
		_ = conn.(*net.UDPConn).SetReadBuffer(65535)
		_ = conn.(*net.UDPConn).SetWriteBuffer(65535)
	}

	return conn, nil
}

func Server(ctx context.Context, parentsHandles []uintptr, secret *memguard.LockedBuffer, logger log15.Logger) (wg *sync.WaitGroup, err error) {
	wg = &sync.WaitGroup{}
	for _, handle := range parentsHandles {
		err = serveOne(ctx, wg, handle, secret, logger)
		if err != nil {
			return nil, err
		}
	}
	return wg, nil
}

func serveOne(ctx context.Context, wg *sync.WaitGroup, parentFD uintptr, secret *memguard.LockedBuffer, logger log15.Logger) error {
	logger = logger.New("class", "binder")
	parentFile := os.NewFile(parentFD, "parent_file")

	c, err := net.FileConn(parentFile)
	if err != nil {
		_ = parentFile.Close()
		return err
	}
	childConn := c.(*net.UnixConn)

	cctx, cancel := context.WithCancel(ctx)
	wg.Add(1)
	go func() {
		<-cctx.Done()
		childConn.Close()
		wg.Done()
	}()

	scanner := bufio.NewScanner(childConn)
	scanner.Split(utils.MakeDecryptSplit(secret))
	schan := make(chan *ExternalConn)
	pchan := make(chan *ExternalPacketConn)
	writer := utils.NewEncryptWriter(childConn, secret)

	wg.Add(1)
	go func() {
		defer wg.Done()
		var smsg string
		for {
			select {
			case <-cctx.Done():
				return
			case bc := <-pchan:
				lnet := strings.SplitN(bc.Addr, ":", 2)[0]
				var connFile *os.File
				var err error
				if lnet == "unixgram" {
					connFile, err = bc.Conn.(*net.UnixConn).File()
				} else {
					connFile, err = bc.Conn.(*net.UDPConn).File()
				}
				bc.Conn.Close()

				if err == nil {
					rights := syscall.UnixRights(int(connFile.Fd()))
					logger.Debug("Sending new connection to child", "uid", bc.Uid, "addr", bc.Addr)
					smsg = fmt.Sprintf("newconn %s %s", bc.Uid, bc.Addr)
					_, _, err := writer.WriteMsgUnix([]byte(smsg), rights, nil)
					//_, _, err := childConn.WriteMsgUnix([]byte(smsg), rights, nil)
					if err != nil {
						logger.Warn("Failed to send FD to binder client", "error", err)
					}
					connFile.Close()
				}
			case bc := <-schan:
				lnet := strings.SplitN(bc.Addr, ":", 2)[0]
				var connFile *os.File
				var err error
				if lnet == "unix" {
					connFile, err = bc.Conn.(*net.UnixConn).File()
				} else {
					connFile, err = bc.Conn.(*net.TCPConn).File()
				}
				bc.Conn.Close()
				if err == nil {
					rights := syscall.UnixRights(int(connFile.Fd()))
					logger.Debug("Sending new connection to child", "uid", bc.Uid, "addr", bc.Addr)
					smsg = fmt.Sprintf("newconn %s %s", bc.Uid, bc.Addr)
					_, _, err := writer.WriteMsgUnix([]byte(smsg), rights, nil)
					//_, _, err := childConn.WriteMsgUnix([]byte(smsg), rights, nil)
					if err != nil {
						logger.Warn("Failed to send FD to binder client", "error", err)
					}
					connFile.Close()
				} else {
					logger.Warn("conn.File() error", "error", err)
				}
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer func() {
			cancel()
			wg.Done()
		}()

		listeners := map[string]net.Listener{}
		var rmsg string
		for scanner.Scan() {
			rmsg = strings.Trim(scanner.Text(), " \r\n")
			command := strings.SplitN(rmsg, " ", 2)[0]
			args := strings.Trim(rmsg[len(command):], " \r\n")
			logger.Debug("Received message", "message", rmsg)

			switch command {
			case "listen":
				logger.Debug("asked to listen", "addr", args)
				for _, addr := range strings.Split(args, " ") {
					lnet := strings.SplitN(addr, ":", 2)[0]
					if IsStream(lnet) {
						l, err := listen(cctx, wg, logger, schan, addr)
						if err == nil {
							_, err := writer.Write([]byte(fmt.Sprintf("confirmlisten %s", addr)))
							if err != nil {
								logger.Warn("Failed to confirm listen to client", "error", err)
								_ = l.Close()
							} else {
								listeners[addr] = l
							}
						} else {
							logger.Warn("Listen error", "error", err, "addr", addr)
							_, _ = writer.Write([]byte(fmt.Sprintf("error %s %s", addr, err.Error())))
						}
					} else {
						c, err := listenPacket(addr)
						if err == nil {
							pchan <- &ExternalPacketConn{Addr: addr, Conn: c, Uid: utils.NewUidString()}
						} else {
							logger.Warn("ListenPacket error", "error", err, "addr", addr)
							_, _ = writer.Write([]byte(fmt.Sprintf("error %s %s", addr, err.Error())))
						}
					}
				}

			case "stoplisten":
				l, ok := listeners[args]
				if ok {
					_ = l.Close()
					delete(listeners, args)
				}
				logger.Debug("Asked to stop listening", "addr", args)
				_, _ = writer.Write([]byte(fmt.Sprintf("stopped %s", args)))

			case "byebye":
				return

			default:
			}
		}
		err = scanner.Err()
		if err != nil {
			logger.Debug("Scanner error", "error", err)
		}
	}()
	return nil
}

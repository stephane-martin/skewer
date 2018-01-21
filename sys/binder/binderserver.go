package binder

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"

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

func BinderListen(ctx context.Context, logger log15.Logger, schan chan *ExternalConn, addr string) (net.Listener, error) {
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

	ctx, cancel := context.WithCancel(ctx)

	go func() {
		<-ctx.Done()
		_ = l.Close()
	}()

	go func() {
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

func BinderPacket(addr string) (conn net.PacketConn, err error) {
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

func Binder(parentsHandles []uintptr, logger log15.Logger) (err error) {
	for _, handle := range parentsHandles {
		err = binderOne(handle, logger)
		if err != nil {
			return err
		}
	}
	return nil
}

func binderOne(parentFD uintptr, logger log15.Logger) error {
	logger = logger.New("class", "binder")
	parentFile := os.NewFile(parentFD, "parent_file")

	c, err := net.FileConn(parentFile)
	if err != nil {
		_ = parentFile.Close()
		return err
	}
	childConn := c.(*net.UnixConn)

	ctx, cancel := context.WithCancel(context.Background())

	schan := make(chan *ExternalConn)
	pchan := make(chan *ExternalPacketConn)

	go func() {
		var smsg string
		for {
			select {
			case <-ctx.Done():
				return
			case bc := <-pchan:
				lnet := strings.SplitN(bc.Addr, ":", 2)[0]
				var connFile *os.File
				var err error
				if lnet == "unixgram" {
					conn := bc.Conn.(*net.UnixConn)
					connFile, err = conn.File()
				} else {
					conn := bc.Conn.(*net.UDPConn)
					connFile, err = conn.File()
				}
				bc.Conn.Close()

				if err == nil {
					rights := syscall.UnixRights(int(connFile.Fd()))
					logger.Debug("Sending new connection to child", "uid", bc.Uid, "addr", bc.Addr)
					smsg = fmt.Sprintf("newconn %s %s\n", bc.Uid, bc.Addr)
					_, _, err := childConn.WriteMsgUnix([]byte(smsg), rights, nil)
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
					conn := bc.Conn.(*net.UnixConn)
					connFile, err = conn.File()
				} else {
					conn := bc.Conn.(*net.TCPConn)
					connFile, err = conn.File()
				}
				bc.Conn.Close()
				if err == nil {
					rights := syscall.UnixRights(int(connFile.Fd()))
					logger.Debug("Sending new connection to child", "uid", bc.Uid, "addr", bc.Addr)
					smsg = fmt.Sprintf("newconn %s %s\n", bc.Uid, bc.Addr)
					_, _, err := childConn.WriteMsgUnix([]byte(smsg), rights, nil)
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

	go func() {
		defer cancel()
		scanner := bufio.NewScanner(childConn)

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
						l, err := BinderListen(ctx, logger, schan, addr)
						if err == nil {
							_, err := childConn.Write([]byte(fmt.Sprintf("confirmlisten %s\n", addr)))
							if err != nil {
								logger.Warn("Failed to confirm listen to client", "error", err)
								_ = l.Close()
							} else {
								listeners[addr] = l
							}
						} else {
							logger.Warn("Listen error", "error", err, "addr", addr)
							_, _ = childConn.Write([]byte(fmt.Sprintf("error %s %s\n", addr, err.Error())))
						}
					} else {
						c, err := BinderPacket(addr)
						if err == nil {
							pchan <- &ExternalPacketConn{Addr: addr, Conn: c, Uid: utils.NewUidString()}
						} else {
							logger.Warn("ListenPacket error", "error", err, "addr", addr)
							_, _ = childConn.Write([]byte(fmt.Sprintf("error %s %s\n", addr, err.Error())))
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
				_, _ = childConn.Write([]byte(fmt.Sprintf("stopped %s\n", args)))

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

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
	"github.com/oklog/ulid"
	"github.com/stephane-martin/skewer/utils"
)

type BinderConn struct {
	Uid  string
	Conn net.Conn
	Addr string
}

type BinderPacketConn struct {
	Uid  string
	Conn net.PacketConn
	Addr string
}

func BinderListen(ctx context.Context, logger log15.Logger, schan chan *BinderConn, generator chan ulid.ULID, addr string) (net.Listener, error) {
	parts := strings.SplitN(addr, ":", 2)
	lnet := parts[0]
	laddr := parts[1]

	l, err := net.Listen(lnet, laddr)

	if err != nil {
		return nil, err
	}

	if lnet == "unix" || lnet == "unixpacket" {
		os.Chmod(laddr, 0777)
		l.(*net.UnixListener).SetUnlinkOnClose(true)
	}

	ctx, cancel := context.WithCancel(ctx)

	go func() {
		<-ctx.Done()
		l.Close()
	}()

	go func() {
		for {
			c, err := l.Accept()
			if err == nil {
				uid := <-generator
				uids := uid.String()
				logger.Debug("New accepted connection", "uid", uids, "addr", addr)
				schan <- &BinderConn{Uid: uids, Conn: c, Addr: addr}
			} else {
				logger.Warn("Accept error", "error", err, "addr", addr)
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
		os.Chmod(laddr, 0777)
		conn.(*net.UnixConn).SetReadBuffer(65536)
		conn.(*net.UnixConn).SetWriteBuffer(65536)
	} else {
		conn.(*net.UDPConn).SetReadBuffer(65535)
		conn.(*net.UDPConn).SetWriteBuffer(65535)
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
		parentFile.Close()
		return err
	}
	childConn := c.(*net.UnixConn)

	ctx, cancel := context.WithCancel(context.Background())

	generator := utils.Generator(ctx, logger)

	schan := make(chan *BinderConn)
	pchan := make(chan *BinderPacketConn)

	go func() {
		var smsg string
		connections := map[string]net.Conn{}
		packetconnections := map[string]net.PacketConn{}
		connfiles := map[string]*os.File{}
		for {
			select {
			case <-ctx.Done():
				for _, conn := range packetconnections {
					conn.Close()
				}
				return
			case bc := <-pchan:
				if bc.Conn == nil {
					if len(bc.Uid) == 0 {
						// close all UDP connections
						for uid := range packetconnections {
							if unixc, unixok := packetconnections[uid].(*net.UnixConn); unixok {
								path := unixc.LocalAddr().String()
								if !strings.HasPrefix(path, "@") {
									os.Remove(path)
								}
							}
							packetconnections[uid].Close()
							if f, ok := connfiles[uid]; ok {
								f.Close()
								delete(connfiles, uid)
							}
						}
						packetconnections = map[string]net.PacketConn{}
					} else {
						// close one UDP connection
						f, ok := connfiles[bc.Uid]
						if ok {
							delete(connfiles, bc.Uid)
							f.Close()
						}
						conn, ok := packetconnections[bc.Uid]
						if ok {
							if unixc, unixok := conn.(*net.UnixConn); unixok {
								path := unixc.LocalAddr().String()
								if !strings.HasPrefix(path, "@") {
									os.Remove(path)
								}
							}
							delete(packetconnections, bc.Uid)
							conn.Close()
						}
					}
				} else {
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
					if err == nil {
						packetconnections[bc.Uid] = bc.Conn
						connfiles[bc.Uid] = connFile
						rights := syscall.UnixRights(int(connFile.Fd()))
						logger.Debug("Sending new connection to child", "uid", bc.Uid, "addr", bc.Addr)
						smsg = fmt.Sprintf("newconn %s %s\n", bc.Uid, bc.Addr)
						childConn.WriteMsgUnix([]byte(smsg), rights, nil)
					}
				}
			case bc := <-schan:
				if bc.Conn == nil {
					if len(bc.Uid) == 0 {
						for uid := range connections {
							connections[uid].Close()
							if f, ok := connfiles[uid]; ok {
								f.Close()
								delete(connfiles, uid)
							}
						}
						connections = map[string]net.Conn{}
					} else {
						f, ok := connfiles[bc.Uid]
						if ok {
							delete(connfiles, bc.Uid)
							f.Close()
						}
						conn, ok := connections[bc.Uid]
						if ok {
							delete(connections, bc.Uid)
							conn.Close()
						}
					}
				} else {
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
					if err == nil {
						connections[bc.Uid] = bc.Conn
						connfiles[bc.Uid] = connFile
						rights := syscall.UnixRights(int(connFile.Fd()))
						logger.Debug("Sending new connection to child", "uid", bc.Uid, "addr", bc.Addr)
						smsg = fmt.Sprintf("newconn %s %s\n", bc.Uid, bc.Addr)
						childConn.WriteMsgUnix([]byte(smsg), rights, nil)
					} else {
						logger.Warn("conn.File() error", "error", err)
					}
				}
			}
		}
	}()

	go func() {
		defer func() {
			cancel()
		}()
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
						l, err := BinderListen(ctx, logger, schan, generator, addr)
						if err == nil {
							listeners[addr] = l
							childConn.Write([]byte(fmt.Sprintf("confirmlisten %s", addr)))
						} else {
							logger.Warn("Listen error", "error", err, "addr", addr)
							childConn.Write([]byte(fmt.Sprintf("error %s %s", addr, err.Error())))
						}
					} else {
						c, err := BinderPacket(addr)
						if err == nil {
							uid := <-generator
							pchan <- &BinderPacketConn{Addr: addr, Conn: c, Uid: uid.String()}
						} else {
							logger.Warn("ListenPacket error", "error", err, "addr", addr)
							childConn.Write([]byte(fmt.Sprintf("error %s %s", addr, err.Error())))
						}
					}
				}
			case "closeconn":
				schan <- &BinderConn{Uid: args}
				pchan <- &BinderPacketConn{Uid: args}

			case "stoplisten":
				l, ok := listeners[args]
				if ok {
					l.Close()
					delete(listeners, args)
				}
				logger.Debug("Asked to stop listening", "addr", args)
				childConn.Write([]byte(fmt.Sprintf("stopped %s\n", args)))
			case "reset":
				for _, l := range listeners {
					l.Close()
				}
				listeners = map[string]net.Listener{}
				schan <- &BinderConn{}
				pchan <- &BinderPacketConn{}

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

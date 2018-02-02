package binder

import "net"

type Client interface {
	Listen(lnet string, laddr string) (net.Listener, error)
	ListenPacket(lnet string, laddr string, bytes int) (net.PacketConn, error)
	StopListen(addr string) error
	Quit() error
}

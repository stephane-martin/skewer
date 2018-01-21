package binder

import "net"

type Client interface {
	Listen(lnet string, laddr string) (net.Listener, error)
	ListenPacket(lnet string, laddr string) (net.PacketConn, error)
	StopListen(addr string) error
	Quit() error
}

// +build linux

package auditlogs

import (
	"bytes"
	"context"
	"encoding/binary"
	"sync/atomic"
	"syscall"
	"time"
)

// Endianness is an alias for what we assume is the current machine endianness
var Endianness = binary.LittleEndian

const (
	// MAX_AUDIT_MESSAGE_LENGTH see http://lxr.free-electrons.com/source/include/uapi/linux/audit.h#L398
	MAX_AUDIT_MESSAGE_LENGTH = 8970
)

type AuditStatusPayload struct {
	Mask            uint32
	Enabled         uint32
	Failure         uint32
	Pid             uint32
	RateLimit       uint32
	BacklogLimit    uint32
	Lost            uint32
	Backlog         uint32
	Version         uint32
	BacklogWaitTime uint32
}

// NetlinkPacket is an alias to give the header a similar name here
type NetlinkPacket syscall.NlMsghdr

type NetlinkClient struct {
	fd      int
	address syscall.Sockaddr
	seq     uint32
	buf     []byte
}

// NewNetlinkClient creates a new NetLinkClient and optionally tries to modify the netlink recv buffer
func NewNetlinkClient(ctx context.Context, recvSize int) (*NetlinkClient, error) {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, syscall.NETLINK_AUDIT)
	if err != nil {
		return nil, err
	}

	n := &NetlinkClient{
		fd:      fd,
		address: &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK, Groups: 0, Pid: 0},
		buf:     make([]byte, MAX_AUDIT_MESSAGE_LENGTH),
	}

	if err = syscall.Bind(fd, n.address); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	// Set the buffer size if we were asked
	if recvSize > 0 {
		if err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, recvSize); err != nil {
			return nil, err
		}
	}

	go func() {
		for {
			n.KeepConnection()
			select {
			case <-ctx.Done():
				syscall.Close(fd)
				return
			case <-time.After(time.Second * 5):
			}
		}
	}()

	return n, nil
}

// Send will send a packet and payload to the netlink socket without waiting for a response
func (n *NetlinkClient) Send(np *NetlinkPacket, a *AuditStatusPayload) error {
	//We need to get the length first. This is a bit wasteful, but requests are rare so yolo..
	buf := new(bytes.Buffer)
	var length int

	np.Seq = atomic.AddUint32(&n.seq, 1)

	for {
		buf.Reset()
		binary.Write(buf, Endianness, np)
		binary.Write(buf, Endianness, a)
		if np.Len == 0 {
			length = len(buf.Bytes())
			np.Len = uint32(length)
		} else {
			break
		}
	}

	return syscall.Sendto(n.fd, buf.Bytes(), 0, n.address)
}

// Receive will receive a packet from a netlink socket
func (n *NetlinkClient) Receive() (*syscall.NetlinkMessage, error) {
	set := syscall.FdSet{}
	set.Bits[n.fd/64] |= (1 << (uint(n.fd) % 64))
	i, _ := syscall.Select(n.fd+1, &set, nil, nil, &syscall.Timeval{Usec: 1000000})
	if i == 0 {
		// select timeout
		return nil, nil
	}
	nlen, _, err := syscall.Recvfrom(n.fd, n.buf, 0)
	if err != nil {
		return nil, err
	}
	if nlen < 1 {
		return nil, nil
	}

	msg := &syscall.NetlinkMessage{
		Header: syscall.NlMsghdr{
			Len:   Endianness.Uint32(n.buf[0:4]),
			Type:  Endianness.Uint16(n.buf[4:6]),
			Flags: Endianness.Uint16(n.buf[6:8]),
			Seq:   Endianness.Uint32(n.buf[8:12]),
			Pid:   Endianness.Uint32(n.buf[12:16]),
		},
		Data: n.buf[syscall.SizeofNlMsghdr:nlen],
	}

	return msg, nil
}

// KeepConnection re-establishes our connection to the netlink socket
func (n *NetlinkClient) KeepConnection() error {
	payload := &AuditStatusPayload{
		Mask:    4,
		Enabled: 1,
		Pid:     uint32(syscall.Getpid()),
		//TODO: Failure: http://lxr.free-electrons.com/source/include/uapi/linux/audit.h#L338
	}

	packet := &NetlinkPacket{
		Type:  uint16(1001),
		Flags: syscall.NLM_F_REQUEST | syscall.NLM_F_ACK,
		Pid:   uint32(syscall.Getpid()),
	}

	return n.Send(packet, payload)
}

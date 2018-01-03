package model

import (
	"github.com/stephane-martin/skewer/utils"
)

type Stasher interface {
	Stash(m *FullMessage) (error, error)
}

type Reporter interface {
	Stasher
	Report(infos []ListenerInfo) error
}

type ListenerInfo struct {
	Port           int    `json:"port" msg:"port"`
	BindAddr       string `json:"bind_addr" mdg:"bind_addr"`
	UnixSocketPath string `json:"unix_socket_path" msg:"unix_socket_path"`
	Protocol       string `json:"protocol" msg:"protocol"`
}

type RawMessage struct {
	Client         string
	LocalPort      int32
	UnixSocketPath string
	Format         string
	Encoding       string
	ConfID         utils.MyULID
}

type RawKafkaMessage struct {
	Brokers    string
	Format     string
	Encoding   string
	ConfID     utils.MyULID
	ConsumerID uint32
	Message    []byte
	UID        utils.MyULID
	Topic      string
	Partition  int32
	Offset     int64
}

type RawTcpMessage struct {
	RawMessage
	Message []byte
	Size    int
	Txnr    int32
	ConnID  uint32
}

type RawUdpMessage struct {
	RawMessage
	Message [65536]byte
	Size    int
}

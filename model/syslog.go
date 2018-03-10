package model

import (
	"github.com/spaolacci/murmur3"
	"github.com/stephane-martin/skewer/utils"
	"github.com/zond/gotomic"
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

type DecoderConfig struct {
	Format    string
	Charset   string
	W3CFields string
}

// TODO: get rid of duplicate type
func (c *DecoderConfig) Equals(other gotomic.Thing) bool {
	if o, ok := other.(*DecoderConfig); ok {
		return *c == *o
	}
	return false
}

func (c *DecoderConfig) HashCode() uint32 {
	h := murmur3.New32()
	h.Write([]byte(c.Format))
	h.Write([]byte(c.Charset))
	h.Write([]byte(c.W3CFields))
	return h.Sum32()
}

type RawFileMessage struct {
	Decoder   DecoderConfig
	Hostname  string
	Directory string
	Glob      string
	Filename  string
	Line      []byte
	ConfID    utils.MyULID
}

type RawMessage struct {
	Decoder        DecoderConfig
	Client         string
	LocalPort      int32
	UnixSocketPath string
	ConfID         utils.MyULID
}

type RawKafkaMessage struct {
	Decoder    DecoderConfig
	Brokers    string
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
	Txnr    int32
	ConnID  utils.MyULID
}

type RawUdpMessage struct {
	RawMessage
	Message [65536]byte
	Size    int
}

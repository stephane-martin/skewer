package server

import (
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	uuid "github.com/satori/go.uuid"
	"github.com/stephane-martin/relp2kafka/conf"
	"github.com/stephane-martin/relp2kafka/model"
	"github.com/stephane-martin/relp2kafka/store"
)

type UdpServerStatus int

const (
	UdpStopped UdpServerStatus = iota
	UdpStarted
)

type UdpServer struct {
	StoreServer
	statusMutex sync.Mutex
	status      UdpServerStatus
	ClosedChan  chan UdpServerStatus
}

func NewUdpServer(c *conf.GlobalConfig, st *store.MessageStore, logger log15.Logger) *UdpServer {
	s := UdpServer{}
	s.protocol = "udp"
	s.stream = false
	s.Conf = *c
	s.connections = map[net.Conn]bool{}
	s.logger = logger.New("class", "UdpServer")
	s.phandler = UdpHandler{Server: &s}
	s.status = UdpStopped
	s.store = st

	return &s
}

func (s *UdpServer) Start() (err error) {
	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()
	if s.status != UdpStopped {
		err = ServerNotStopped
		return
	}
	s.ClosedChan = make(chan UdpServerStatus, 1)

	s.packetConnections = map[net.PacketConn]bool{}
	nb := s.ListenPacket()
	if nb > 0 {
		s.status = UdpStarted
		//s.storeToKafkaWg.Add(1)
		//go s.Store2Kafka()
	} else {
		s.logger.Info("The UDP service has not been started: no listening port")
		close(s.ClosedChan)
	}
	return
}

func (s *UdpServer) Stop() {
	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()
	if s.status != UdpStarted {
		return
	}
	s.logger.Debug("Closing UDP connections")
	s.CloseConnections()
	s.logger.Debug("Waiting for UDP goroutines")
	s.wg.Wait()
	s.logger.Debug("UdpServer goroutines have ended")
	//s.store.StopSend()
	//s.logger.Debug("Waiting for the Store to finish operations")
	//s.storeToKafkaWg.Wait()

	s.status = UdpStopped
	s.ClosedChan <- UdpStopped
	close(s.ClosedChan)
	s.logger.Info("Udp server has stopped")
}

type UdpHandler struct {
	Server *UdpServer
}

func (h UdpHandler) HandleConnection(conn net.PacketConn, i int) {
	s := h.Server
	s.AddPacketConnection(conn)

	raw_messages_chan := make(chan *model.TcpUdpRawMessage)

	defer func() {
		close(raw_messages_chan)
		s.RemovePacketConnection(conn)
		s.wg.Done()
	}()

	var local_port int
	local := conn.LocalAddr()
	if local != nil {
		s := strings.Split(local.String(), ":")
		local_port, _ = strconv.Atoi(s[len(s)-1])
	}

	logger := s.logger.New("local_port", local_port)

	// pull messages from raw_messages_chan, parse them and push them to the Store
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for m := range raw_messages_chan {
			p, err := model.Parse(m.Message, s.Conf.Syslog[i].Format, s.Conf.Syslog[i].DontParseSD)

			if err == nil {
				// todo: get rid of pointer to time
				t := time.Now()
				if p.TimeReported != nil {
					t = *p.TimeReported
				} else {
					p.TimeReported = &t
					p.TimeGenerated = &t
				}
				uid := t.Format(time.RFC3339) + m.Uid.String()
				parsed_msg := model.TcpUdpParsedMessage{
					Parsed: model.ParsedMessage{
						Fields:    p,
						Client:    m.Client,
						LocalPort: m.LocalPort,
					},
					Uid:       uid,
					ConfIndex: i,
				}
				s.store.Inputs <- &parsed_msg
			} else {
				logger.Info("Parsing error", "Message", m.Message, "error", err)
			}
		}
	}()

	// Syslog UDP server
	for {
		packet := make([]byte, 65536)
		size, remote, err := conn.ReadFrom(packet)
		if err != nil {
			logger.Info("Error reading UDP", "error", err)
			return
		}
		client := strings.Split(remote.String(), ":")[0]
		raw := model.TcpUdpRawMessage{
			RawMessage: model.RawMessage{
				Client:    client,
				LocalPort: local_port,
				Message:   string(packet[:size]),
			},
			Uid: uuid.NewV4(),
		}
		raw_messages_chan <- &raw
	}

}

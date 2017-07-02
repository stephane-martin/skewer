package server

import (
	"bufio"
	"bytes"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/relp2kafka/conf"
	"github.com/stephane-martin/relp2kafka/metrics"
	"github.com/stephane-martin/relp2kafka/model"
	"github.com/stephane-martin/relp2kafka/store"
)

type TcpServerStatus int

const (
	TcpStopped TcpServerStatus = iota
	TcpStarted
)

type TcpServer struct {
	StreamServer
	status     TcpServerStatus
	ClosedChan chan TcpServerStatus
	store      *store.MessageStore
	metrics    *metrics.Metrics
}

func (s *TcpServer) init() {
	s.StreamServer.init()
}

func NewTcpServer(c *conf.GConfig, st *store.MessageStore, metric *metrics.Metrics, logger log15.Logger) *TcpServer {
	s := TcpServer{
		status:  TcpStopped,
		store:   st,
		metrics: metric,
	}
	s.logger = logger.New("class", "TcpServer")
	s.protocol = "tcp"
	s.Conf = *c
	s.init()
	s.handler = TcpHandler{Server: &s}
	return &s
}

func (s *TcpServer) Start() error {
	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()
	if s.status != TcpStopped {
		return ServerNotStopped
	}
	s.ClosedChan = make(chan TcpServerStatus, 1)

	// start listening on the required ports
	nb := s.initTCPListeners()
	if nb > 0 {
		s.status = TcpStarted
		s.Listen()
	} else {
		s.logger.Info("TCP Server not started: no listening port")
		close(s.ClosedChan)
	}
	return nil
}

func (s *TcpServer) Stop() {
	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()
	if s.status != TcpStarted {
		return
	}
	s.resetTCPListeners() // close the listeners. This will make Listen to return and close all current connections.
	s.wg.Wait()           // wait that all HandleConnection goroutines have ended
	s.logger.Debug("TcpServer goroutines have ended")

	s.status = TcpStopped
	s.ClosedChan <- TcpStopped
	close(s.ClosedChan)
	s.logger.Info("TCP server has stopped")
}

type TcpHandler struct {
	Server *TcpServer
}

func (h TcpHandler) HandleConnection(conn net.Conn, config conf.SyslogConfig) {

	var local_port int

	s := h.Server
	s.AddConnection(conn)

	raw_messages_chan := make(chan *model.RawMessage)

	defer func() {
		close(raw_messages_chan)
		s.RemoveConnection(conn)
		s.wg.Done()
	}()

	configId, err := s.store.StoreSyslogConfig(&config)
	if err != nil {
		s.logger.Error("Error storing configuration", "error", err)
		return
	}

	client := ""
	path := ""
	remote := conn.RemoteAddr()

	if remote == nil {
		client = "localhost"
		local_port = 0
		path = conn.LocalAddr().String()
	} else {
		client = strings.Split(remote.String(), ":")[0]
		local := conn.LocalAddr()
		if local != nil {
			s := strings.Split(local.String(), ":")
			local_port, _ = strconv.Atoi(s[len(s)-1])
		}
	}
	client = strings.TrimSpace(client)
	path = strings.TrimSpace(path)
	local_port_s := strconv.FormatInt(int64(local_port), 10)

	logger := s.logger.New("protocol", s.protocol, "client", client, "local_port", local_port, "unix_socket_path", path)
	logger.Info("New client")
	s.metrics.ClientConnectionCounter.WithLabelValues(s.protocol, client, local_port_s, path).Inc()

	// pull messages from raw_messages_chan, parse them and push them to the Store
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		e := s.NewParsersEnv()
		entropy := rand.New(rand.NewSource(time.Now().UnixNano()))
		for m := range raw_messages_chan {
			parser := e.GetParser(config.Format)
			if parser == nil {
				s.logger.Error("Unknown parser", "client", m.Client, "local_port", m.LocalPort, "path", m.UnixSocketPath, "format", config.Format)
				continue
			}
			p, err := parser.Parse(m.Message, config.DontParseSD)

			if err == nil {
				uid, err := ulid.New(ulid.Timestamp(p.TimeReported), entropy)
				if err != nil {
					// should not happen
					s.logger.Error("Error generating a ULID", "error", err)
				} else {
					parsed_msg := model.TcpUdpParsedMessage{
						Parsed: model.ParsedMessage{
							Fields:         p,
							Client:         m.Client,
							LocalPort:      m.LocalPort,
							UnixSocketPath: m.UnixSocketPath,
						},
						Uid:    uid.String(),
						ConfId: configId,
					}
					s.store.Inputs <- &parsed_msg
				}
			} else {
				logger.Info("Parsing error", "Message", m.Message, "error", err)
			}
		}
	}()

	timeout := config.Timeout
	if timeout > 0 {
		conn.SetReadDeadline(time.Now().Add(timeout))
	}
	scanner := bufio.NewScanner(conn)
	switch config.Format {
	case "rfc5424", "rfc3164", "json", "auto":
		scanner.Split(TcpSplit)
	default:
		scanner.Split(LFTcpSplit)
	}

	for {
		if scanner.Scan() {
			if timeout > 0 {
				conn.SetReadDeadline(time.Now().Add(timeout))
			}
			raw := model.RawMessage{
				Client:    client,
				LocalPort: local_port,
				Message:   scanner.Text(),
			}
			s.metrics.IncomingMsgsCounter.WithLabelValues(s.protocol, client, local_port_s, path).Inc()
			raw_messages_chan <- &raw
		} else {
			logger.Info("Scanning the TCP stream has ended", "error", scanner.Err())
			return
		}
	}
}

func LFTcpSplit(data []byte, atEOF bool) (int, []byte, error) {
	trimmed_data := bytes.TrimLeft(data, " \r\n")
	if len(trimmed_data) == 0 {
		return 0, nil, nil
	}
	trimmed := len(data) - len(trimmed_data)
	lf := bytes.IndexByte(trimmed_data, '\n')
	if lf >= 0 {
		token := bytes.Trim(trimmed_data[0:lf], " \r\n")
		advance := trimmed + lf + 1
		return advance, token, nil
	} else {
		// data does not contain a full syslog line
		return 0, nil, nil
	}
}

func TcpSplit(data []byte, atEOF bool) (int, []byte, error) {
	trimmed_data := bytes.TrimLeft(data, " \r\n")
	if len(trimmed_data) == 0 {
		return 0, nil, nil
	}
	trimmed := len(data) - len(trimmed_data)
	if trimmed_data[0] == byte('<') {
		// LF framing
		lf := bytes.IndexByte(trimmed_data, '\n')
		if lf >= 0 {
			token := bytes.Trim(trimmed_data[0:lf], " \r\n")
			advance := trimmed + lf + 1
			return advance, token, nil
		} else {
			// data does not contain a full syslog line
			return 0, nil, nil
		}
	} else {
		// octet counting framing
		sp := bytes.IndexAny(trimmed_data, " \n")
		if sp <= 0 {
			return 0, nil, nil
		}
		datalen_s := bytes.Trim(trimmed_data[0:sp], " \r\n")
		datalen, err := strconv.Atoi(string(datalen_s))
		if err != nil {
			return 0, nil, err
		}
		advance := trimmed + sp + 1 + datalen
		if len(data) >= advance {
			token := bytes.Trim(trimmed_data[sp+1:sp+1+datalen], " \r\n")
			return advance, token, nil
		} else {
			return 0, nil, nil
		}

	}
}

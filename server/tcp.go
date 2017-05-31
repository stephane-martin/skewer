package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	sarama "gopkg.in/Shopify/sarama.v1"

	"github.com/inconshreveable/log15"
	"github.com/satori/go.uuid"
	"github.com/stephane-martin/relp2kafka/conf"
	"github.com/stephane-martin/relp2kafka/model"
	"github.com/stephane-martin/relp2kafka/store"
)

type TcpServerStatus int

const (
	TcpStopped TcpServerStatus = iota
	TcpStarted
)

type TcpServer struct {
	Server
	statusMutex sync.Mutex
	status      TcpServerStatus
	StatusChan  chan TcpServerStatus
	store       *store.MessageStore
	store_wg    sync.WaitGroup
	producer    sarama.AsyncProducer
}

func NewTcpServer(c *conf.GlobalConfig, logger log15.Logger) *TcpServer {
	s := TcpServer{}
	s.protocol = "tcp"
	s.Conf = c
	s.listeners = map[int]net.Listener{}
	s.connections = map[net.Conn]bool{}
	s.logger = logger.New("class", "TcpServer")
	s.handler = TcpHandler{Server: &s}
	s.status = TcpStopped

	return &s
}

func (s *TcpServer) Start() (err error) {
	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()
	if s.status != TcpStopped {
		err = ServerNotStopped
		return
	}
	s.StatusChan = make(chan TcpServerStatus, 1)

	// initialize the store
	s.store, err = store.NewStore(s.Conf.Store.Dirname, s.logger)
	if err != nil {
		close(s.StatusChan)
		return
	}

	// start listening on the required ports
	err = s.initListeners()
	if err != nil {
		// net.OpError
		s.store.StopSend()
		close(s.StatusChan)
		return
	}
	s.status = TcpStarted
	s.wg.Add(1)
	go s.Listen()

	s.store_wg.Add(1)
	go s.Store2Kafka()

	return
}

func (s *TcpServer) Stop() {
	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()
	s.logger.Debug("Stop")
	if s.status != TcpStarted {
		return
	}
	s.resetListeners() // close the listeners. This will make Listen to return and close each current connection with a client
	// wait that all HandleConnection goroutines have ended
	s.wg.Wait()
	s.logger.Debug("TcpServer goroutines have ended")
	// from here there will be no more incoming TCP messages to store: it is safe to stop the Store
	s.store.StopSend()
	// StopSend closes the Store.Inputs channel
	// store.startIngest will then stop. No more messages will be stashed in the Store,
	// and no more messages will arrive in the Store.toProcess queue
	// eventually Store.retrieve will return an empty result, and store.StartSend will end
	// after that the Store.Outputs channel will be closed (in the defer)
	// Store2Kafka will end, because of the closed Store.Outputs channel
	// The Kafka producer is then closed (AsyncClose in defer)
	// The child goroutine is Store2Kafka will drain the Producer Successes and Errors channels, and then return
	// After the drain, the Store is finally closed (in defer), and store_wg.Wait() will return
	s.logger.Debug("Waiting for the Store to finish operations")
	s.store_wg.Wait()

	s.status = TcpStopped
	s.StatusChan <- TcpStopped
	close(s.StatusChan)
	s.logger.Info("TCP server has stopped")
}

func (s *TcpServer) Store2Kafka() {
	defer s.store_wg.Done()
	s.logger.Debug("Store2Kafka")
	if s.test {
		s.store.StartSend()
		s.logger.Debug("Test mode: we don't really send messages to Kafka, we just print them")
		for message := range s.store.Outputs {
			if message != nil {
				fmt.Println(message.Message)
				s.store.Ack(message.Uid)
			}
		}
		s.logger.Debug("Consumed all messages coming from the Store")
		s.store.Close()
	} else {
		var producer sarama.AsyncProducer
		var err error
		for !s.store.Stopped() {
			producer, err = s.Conf.GetKafkaAsyncProducer()
			if err == nil {
				break
			} else {
				// todo: wait a bit
				s.logger.Warn("Error getting a Kafka client", "error", err)
			}
		}
		if producer == nil {
			return
		}
		defer producer.AsyncClose()

		// listen for kafka NACK responses
		s.store_wg.Add(1)
		go func() {
			defer func() {
				s.store.Close()
				s.store_wg.Done()
			}()
			more_succs := true
			more_fails := true
			var succ *sarama.ProducerMessage
			var fail *sarama.ProducerError
			for more_succs || more_fails {
				select {
				case succ, more_succs = <-producer.Successes():
					if more_succs {
						uid := succ.Metadata.(string)
						s.store.Ack(uid)
					}

				case fail, more_fails = <-producer.Errors():
					if more_fails {
						uid := fail.Msg.Metadata.(string)
						s.store.Nack(uid)
					}
				}
			}
		}()

		s.store.StartSend()
		for message := range s.store.Outputs {
			value, err := json.Marshal(message)
			if err != nil {
				s.logger.Warn("Error marshaling a message to JSON", "error", err)
				continue
			}
			partitionKeyBuf := bytes.Buffer{}
			err = s.Conf.Syslog[message.ConfIndex].PartitionKeyTemplate.Execute(&partitionKeyBuf, message)
			if err != nil {
				s.logger.Warn("Error generating the partition hash key", "error", err)
				continue
			}
			topicBuf := bytes.Buffer{}
			err = s.Conf.Syslog[message.ConfIndex].TopicTemplate.Execute(&topicBuf, message)
			if err != nil {
				s.logger.Warn("Error generating the topic", "error", err)
				continue
			}

			kafka_msg := sarama.ProducerMessage{
				Key:       sarama.ByteEncoder(partitionKeyBuf.Bytes()),
				Value:     sarama.ByteEncoder(value),
				Topic:     topicBuf.String(),
				Timestamp: *message.Message.TimeReported,
				Metadata:  message.Uid,
			}
			producer.Input() <- &kafka_msg
		}
	}
}

type TcpHandler struct {
	Server *TcpServer
}

func (h TcpHandler) HandleConnection(conn net.Conn, i int) {
	s := h.Server
	s.AddConnection(conn)

	raw_messages_chan := make(chan *model.TcpRawMessage)

	var client string
	remote := conn.RemoteAddr()
	if remote != nil {
		client = strings.Split(remote.String(), ":")[0]
	}

	var local_port int
	local := conn.LocalAddr()
	if local != nil {
		s := strings.Split(local.String(), ":")
		local_port, _ = strconv.Atoi(s[len(s)-1])
	}

	logger := s.logger.New("remote", client, "local_port", local_port)
	logger.Info("New TCP client")

	// pull messages from raw_messages_chan, parse them and push them to the Store
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for m := range raw_messages_chan {
			p, err := model.Parse(m.Message, s.Conf.Syslog[i].Format)
			t := time.Now()
			if p.TimeReported != nil {
				t = *p.TimeReported
			} else {
				p.TimeReported = &t
				p.TimeGenerated = &t
			}
			if err == nil {
				uid := t.Format(time.RFC3339) + m.Uid.String()
				parsed_msg := model.TcpParsedMessage{Message: p, Uid: uid, Client: m.Client, LocalPort: m.LocalPort, ConfIndex: i}
				s.store.Inputs <- &parsed_msg
			} else {
				logger.Info("Parsing error", "Message", m.Message)
			}
		}
	}()

	defer func() {
		close(raw_messages_chan)
		s.RemoveConnection(conn)
		s.wg.Done()
	}()

	// Syslog TCP server
	scanner := bufio.NewScanner(conn)
	scanner.Split(TcpSplit)

	for {
		if scanner.Scan() {
			data := scanner.Text()

			raw := model.TcpRawMessage{
				RawMessage: model.RawMessage{
					Client:    client,
					LocalPort: local_port,
					Message:   data,
				},
				Uid: uuid.NewV4(),
			}
			raw_messages_chan <- &raw
		} else {
			logger.Info("Scanning the TCP stream has ended", "error", scanner.Err())
			return
		}
	}
}

func TcpSplit(data []byte, atEOF bool) (int, []byte, error) {
	trimmed_data := bytes.TrimLeft(data, " \r\n")
	if len(trimmed_data) == 0 {
		return 0, nil, nil
	}
	trimmed := len(data) - len(trimmed_data)
	if trimmed_data[0] == byte('<') {
		// non-transparent-framing
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

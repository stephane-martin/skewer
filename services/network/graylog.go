package network

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Graylog2/go-gelf/gelf"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/decoders"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/utils"
)

type GraylogStatus int

const (
	GraylogStopped GraylogStatus = iota
	GraylogStarted
)

var (
	magicChunked = []byte{0x1e, 0x0f}
	magicZlib    = []byte{0x78}
	magicGzip    = []byte{0x1f, 0x8b}
)

const (
	chunkedHeaderLen = 12
	chunkedDataLen   = gelf.ChunkSize - chunkedHeaderLen
)

func initGraylogRegistry() {
	base.Once.Do(func() {
		base.InitRegistry()
	})
}

type GraylogSvcImpl struct {
	base.BaseService
	Configs        []conf.GraylogSourceConfig
	status         GraylogStatus
	stasher        *base.Reporter
	wg             sync.WaitGroup
	fatalErrorChan chan struct{}
	fatalOnce      *sync.Once
	readers        map[*gelf.Reader]bool
	readersMu      sync.Mutex
}

func NewGraylogService(env *base.ProviderEnv) (base.Provider, error) {
	initGraylogRegistry()
	s := GraylogSvcImpl{
		status:  GraylogStopped,
		stasher: env.Reporter,
		Configs: []conf.GraylogSourceConfig{},
	}
	s.BaseService.Init()
	s.BaseService.Logger = env.Logger.New("class", "GraylogService")
	s.BaseService.Binder = env.Binder
	return &s, nil
}

func (s *GraylogSvcImpl) Type() base.Types {
	return base.Graylog
}

func (s *GraylogSvcImpl) SetConf(c conf.BaseConfig) {
	s.Configs = c.GraylogSource
}

func (s *GraylogSvcImpl) Gather() ([]*dto.MetricFamily, error) {
	return base.Registry.Gather()
}

func (s *GraylogSvcImpl) Start() (infos []model.ListenerInfo, err error) {
	s.LockStatus()
	defer s.UnlockStatus()
	if s.status != GraylogStopped {
		return nil, ServerNotStopped
	}
	s.fatalErrorChan = make(chan struct{})
	s.fatalOnce = &sync.Once{}
	s.ClearConnections()
	infos = s.ListenPacket()
	if len(infos) > 0 {
		s.status = GraylogStarted
		s.Logger.Info("Listening on UDP", "nb_services", len(infos))
	} else {
		s.Logger.Debug("The UDP service has not been started: no listening port")
	}
	return infos, nil
}

func (s *GraylogSvcImpl) FatalError() chan struct{} {
	return s.fatalErrorChan
}

func (s *GraylogSvcImpl) dofatal() {
	s.fatalOnce.Do(func() { close(s.fatalErrorChan) })
}

func (s *GraylogSvcImpl) Shutdown() {
	s.Stop()
}

func (s *GraylogSvcImpl) Stop() {
	s.LockStatus()
	defer s.UnlockStatus()
	if s.status != GraylogStarted {
		return
	}
	s.CloseConnections()
	s.wg.Wait()
	s.status = GraylogStopped
	s.Logger.Debug("Graylog service has stopped")
}

func (s *GraylogSvcImpl) ListenPacket() []model.ListenerInfo {
	infos := []model.ListenerInfo{}
	s.UnixSocketPaths = []string{}
	for _, syslogConf := range s.Configs {
		if len(syslogConf.UnixSocketPath) > 0 {
			conn, err := s.Binder.ListenPacket("unixgram", syslogConf.UnixSocketPath, 65536)
			if err != nil {
				s.Logger.Warn("Listen unixgram error", "error", err)
			} else {
				s.Logger.Debug(
					"Graylog listener",
					"protocol", "graylog",
					"path", syslogConf.UnixSocketPath,
					"format", syslogConf.Format,
				)
				infos = append(infos, model.ListenerInfo{
					UnixSocketPath: syslogConf.UnixSocketPath,
					Protocol:       "graylog",
				})
				s.UnixSocketPaths = append(s.UnixSocketPaths, syslogConf.UnixSocketPath)
				s.wg.Add(1)
				go s.handleConnection(conn, syslogConf)
			}
		} else {
			listenAddrs, _ := syslogConf.GetListenAddrs()
			for port, listenAddr := range listenAddrs {
				conn, err := s.Binder.ListenPacket("udp", listenAddr, 65536)
				if err != nil {
					s.Logger.Warn("Listen UDP error", "error", err)
				} else {
					s.Logger.Debug(
						"Graylog listener",
						"protocol", "graylog",
						"bind_addr", syslogConf.BindAddr,
						"port", port,
						"format", syslogConf.Format,
					)
					infos = append(infos, model.ListenerInfo{
						BindAddr: syslogConf.BindAddr,
						Port:     port,
						Protocol: "graylog",
					})
					s.wg.Add(1)
					go s.handleConnection(conn, syslogConf)
				}
			}
		}
	}
	return infos
}

func (s *GraylogSvcImpl) handleConnection(conn net.PacketConn, config conf.GraylogSourceConfig) {
	s.AddConnection(conn)
	defer func() {
		s.RemoveConnection(conn)
		s.wg.Done()
	}()

	var localPort int
	var localPortS string
	var path string
	var err error
	var full *model.FullMessage
	var n int
	var addr net.Addr
	var client string

	chunks := map[[8]byte](map[uint8]([]byte)){}
	chunkStartTime := map[[8]byte]time.Time{}
	gen := utils.NewGenerator()

	local := conn.LocalAddr()
	if local != nil {
		l := local.String()
		s := strings.Split(l, ":")
		localPort, err = strconv.Atoi(s[len(s)-1])
		if err != nil {
			path = strings.TrimSpace(l)
		} else {
			localPortS = strconv.FormatInt(int64(localPort), 10)
		}
	}

	logger := s.Logger.New("protocol", "graylog", "local_port", localPortS, "unix_socket_path", path)

	cBuf := make([]byte, gelf.ChunkSize)
	for {
		n, addr, err = conn.ReadFrom(cBuf)
		if err != nil {
			logger.Info("Error reading UDP Graylog", "error", err)
			return
		}
		if n < 2 {
			logger.Warn("GELF message was too short", "size", n)
			continue
		}
		cHead := cBuf[:2]
		if bytes.Equal(cHead, magicChunked) {
			if n < 12 {
				logger.Warn("GELF chunk was too short", "size", n)
				continue
			}
			if n == 12 {
				continue
			}
			var msgid [8]byte
			copy(msgid[:], cBuf[2:10])
			seq, total := cBuf[10], cBuf[11]
			if total > 128 {
				logger.Warn("Too many GELF chunks", "total", total)
				delete(chunks, msgid)
				continue
			}
			if seq >= total {
				logger.Warn("Out of band GELF sequence number", "seq", seq, "total", total)
				delete(chunks, msgid)
				continue
			}

			if _, ok := chunks[msgid]; !ok {
				chunks[msgid] = map[uint8]([]byte){}
				chunkStartTime[msgid] = time.Now()

			}
			if (time.Now().Sub(chunkStartTime[msgid])) > (5 * time.Second) {
				logger.Warn("GELF chunk arrived after 5 seconds")
				delete(chunks, msgid)
				continue
			}
			chunks[msgid][seq] = make([]byte, n-chunkedHeaderLen)
			copy(chunks[msgid][seq], cBuf[chunkedHeaderLen:n])
			if len(chunks[msgid]) < int(total) {
				continue
			}
			// rebuild message
			full, err = fromChunks(chunks[msgid], total)
			delete(chunks, msgid)
		} else {
			full, err = fullMsg(cBuf[:n])
		}

		client = "localhost"
		if addr != nil {
			client = strings.Split(addr.String(), ":")[0]
		}

		if err != nil {
			base.CountParsingError(base.Graylog, client, "graylog")
			logger.Warn("Error decoding full GELF message", "error", err)
			continue
		}

		full.Uid = gen.Uid()
		full.ConfId = config.ConfID
		full.SourceType = "graylog"
		full.SourcePath = path
		full.SourcePort = int32(localPort)
		full.ClientAddr = client
		s.stasher.Stash(full)
		base.CountIncomingMessage(base.Graylog, client, localPort, path)
		model.FullFree(full)
	}
}

func fromChunks(chunks map[uint8]([]byte), total uint8) (*model.FullMessage, error) {
	var i uint8
	full := make([]byte, 0, int(total)*gelf.ChunkSize)
	for i = 0; i < total; i++ {
		chunk, ok := chunks[i]
		if !ok {
			return nil, fmt.Errorf("Missing chunk")
		}
		full = append(full, chunk...)
	}
	return fullMsg(full)
}

func fullMsg(buf []byte) (full *model.FullMessage, err error) {
	if len(buf) < 2 {
		return nil, fmt.Errorf("GELF message was too short")
	}
	head := buf[:2]
	var reader io.Reader
	// the data we get from the wire is compressed
	if bytes.Equal(head, magicGzip) {
		reader, err = gzip.NewReader(bytes.NewReader(buf))
	} else if head[0] == magicZlib[0] && (int(head[0])*256+int(head[1]))%31 == 0 {
		reader, err = zlib.NewReader(bytes.NewReader(buf))
	} else {
		// compliance with https://github.com/Graylog2/graylog2-server
		// treating all messages as uncompressed if  they are not gzip, zlib or
		// chunked
		reader = bytes.NewReader(buf)
	}

	if err != nil {
		return nil, fmt.Errorf("NewReader: %s", err)
	}

	gelfmsg := &gelf.Message{}
	if err := json.NewDecoder(reader).Decode(gelfmsg); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %s", err)
	}
	return decoders.FullFromGelfMessage(gelfmsg), nil
}

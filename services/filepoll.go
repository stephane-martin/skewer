package services

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/gobwas/glob"
	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/gotail/tail"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/decoders"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
)

func initPollingRegistry() {
	base.Once.Do(func() {
		base.InitRegistry()
	})
}

type FilePollingService struct {
	stasher        base.Stasher
	logger         log15.Logger
	confs          map[utils.MyULID](*conf.FilesystemSourceConfig)
	confsMap       map[ulid.ULID]utils.MyULID
	parserEnv      *decoders.ParsersEnv
	tailor         *tail.Tailor
	rawQueue       chan *model.RawFileMessage
	fatalErrorChan chan struct{}
	fatalOnce      *sync.Once
	confined       bool
	wg             sync.WaitGroup
	registryOnce   sync.Once
	nWatchedFiles  prometheus.GaugeFunc
	nWatchedDirs   prometheus.GaugeFunc
}

var fpool = &sync.Pool{
	New: func() interface{} {
		return &model.RawFileMessage{}
	},
}

func getFRaw() *model.RawFileMessage {
	return fpool.Get().(*model.RawFileMessage)
}

func freeFRaw(raw *model.RawFileMessage) {
	fpool.Put(raw)
}

func NewFilePollingService(env *base.ProviderEnv) (base.Provider, error) {
	initPollingRegistry()
	s := FilePollingService{
		stasher:  env.Reporter,
		logger:   env.Logger.New("class", "filepoll"),
		confined: env.Confined,
	}
	s.nWatchedFiles = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "skw_filepoll_nfiles",
			Help: "number of watched files",
		},
		s.nFiles,
	)

	s.nWatchedDirs = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "skw_filepoll_ndirs",
			Help: "number of watched directories",
		},
		s.nDirs,
	)
	return &s, nil
}

func (s *FilePollingService) Type() base.Types {
	return base.Filesystem
}

func (s *FilePollingService) Gather() ([]*dto.MetricFamily, error) {
	return base.Registry.Gather()
}

func (s *FilePollingService) FatalError() chan struct{} {
	return s.fatalErrorChan
}

func (s *FilePollingService) dofatal() {
	s.fatalOnce.Do(func() { close(s.fatalErrorChan) })
}

func (s *FilePollingService) nFiles() float64 {
	if s == nil {
		return 0
	}
	if s.tailor == nil {
		return 0
	}
	return float64(s.tailor.NFiles())
}

func (s *FilePollingService) nDirs() float64 {
	if s == nil {
		return 0
	}
	if s.tailor == nil {
		return 0
	}
	return float64(s.tailor.NDirectories())
}

func (s *FilePollingService) Start() (infos []model.ListenerInfo, err error) {
	infos = []model.ListenerInfo{}
	s.fatalErrorChan = make(chan struct{})
	s.fatalOnce = &sync.Once{}
	rawQueue := make(chan *model.RawFileMessage)

	lines := make(chan tail.FileLineID)
	errors := make(chan error)
	tailor, err := tail.NewTailor(lines, errors)
	if err != nil {
		return infos, err
	}
	s.tailor = tailor

	// TODO
	s.registryOnce.Do(func() {
		base.Registry.MustRegister(s.nWatchedFiles, s.nWatchedDirs)
	})

	for _, config := range s.confs {
		filter, err := MakeFilter(config.Glob)
		if err != nil {
			return infos, err
		}
		d := config.BaseDirectory
		if s.confined {
			d = filepath.Join("/tmp", "polldirs", d)
		}
		dirUID, err := tailor.AddRecursiveDirectory(d, filter)
		if err == nil {
			s.confsMap[dirUID] = config.ConfID
		} else {
			s.logger.Warn("Error adding directory to watch", "error", err, "directory", config.BaseDirectory)
		}
	}

	if len(s.confsMap) == 0 {
		return infos, fmt.Errorf("filepoll does not watch any directory")
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.fetchLines(lines, rawQueue)
	}()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		fetchErrors(s.logger, errors)
	}()
	cpus := runtime.NumCPU()
	for i := 0; i < cpus; i++ {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			err := s.parse(rawQueue)
			if err != nil {
				s.dofatal()
				s.logger.Error(err.Error())
			}
		}()
	}

	return infos, nil
}

func flogg(logger log15.Logger, raw *model.RawFileMessage) log15.Logger {
	return logger.New(
		"protocol", "filepoll",
		"format", raw.Decoder.Format,
		"filename", raw.Filename,
	)
}

func (s *FilePollingService) parseOne(raw *model.RawFileMessage, gen *utils.Generator) error {
	parser, err := s.parserEnv.GetParser(&raw.Decoder)
	if parser == nil || err != nil {
		return decoders.DecodingError(eerrors.Wrapf(err, "Unknown decoder: %s", raw.Decoder.Format))
	}
	defer parser.Release()

	syslogMsgs, err := parser.Parse(raw.Line)
	if err != nil {
		return decoders.DecodingError(eerrors.Wrap(err, "Parsing error"))
	}

	var syslogMsg *model.SyslogMessage
	var full *model.FullMessage

	for _, syslogMsg = range syslogMsgs {
		if syslogMsg == nil {
			continue
		}
		syslogMsg.SetProperty("skewer", "client", raw.Hostname)
		syslogMsg.SetProperty("skewer", "filename", raw.Filename)
		syslogMsg.SetProperty("skewer", "directory", raw.Directory)

		full = model.FullFactoryFrom(syslogMsg)
		full.Uid = gen.Uid()
		full.ConfId = raw.ConfID
		err := s.stasher.Stash(full)

		model.FullFree(full)

		if err != nil {
			flogg(s.logger, raw).Error("Error stashing filepoll message", "error", err)
			if eerrors.IsFatal(err) {
				return eerrors.Wrap(err, "Fatal error pushing filepoll message to the Store")
			}
		}
	}
	return nil
}

func (s *FilePollingService) parse(rawq chan *model.RawFileMessage) error {
	gen := utils.NewGenerator()

	for raw := range rawq {
		if raw == nil {
			return nil
		}
		err := s.parseOne(raw, gen)
		if err != nil {
			base.ParsingErrorCounter.WithLabelValues("filepoll", raw.Hostname, raw.Decoder.Format).Inc()
			flogg(s.logger, raw).Warn(err.Error())
		}
		freeFRaw(raw)
		if err != nil && eerrors.IsFatal(err) {
			// stop processing when fatal error happens
			return err
		}
	}
	return nil
}

func fetchErrors(logger log15.Logger, errors chan error) {
	for err := range errors {
		switch e := err.(type) {
		case *tail.FileErrorID:
			logger.Warn("Error watching file", "error", e.Err, "filename", e.Filename)
		case *tail.FileError:
			logger.Warn("Error watching file", "error", e.Err, "filename", e.Filename)
		default:
			logger.Warn("Error watching file", "error", err)
		}
	}
}

func (s *FilePollingService) fetchLines(lines chan tail.FileLineID, rawq chan *model.RawFileMessage) {
	defer close(rawq)
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	for l := range lines {
		config := s.confs[s.confsMap[l.Uid]]
		raw := getFRaw()
		raw.Hostname = hostname
		raw.Decoder = config.DecoderBaseConfig
		raw.Directory = config.BaseDirectory
		raw.Glob = config.Glob
		raw.Filename = l.Filename
		if s.confined && len(raw.Filename) >= 13 {
			raw.Filename = raw.Filename[13:] // /tmp/polldirs/...
		}
		raw.Line = l.Line
		raw.ConfID = config.ConfID
		base.IncomingMsgsCounter.WithLabelValues("filepoll", hostname, "0", config.BaseDirectory)
		rawq <- raw
	}
}

func (s *FilePollingService) Stop() {
	if s.tailor != nil {
		s.tailor.Close()
		s.tailor = nil
	}
	s.wg.Wait()
}

func (s *FilePollingService) Shutdown() {
	s.Stop()
}

func (s *FilePollingService) SetConf(c conf.BaseConfig) {
	s.confs = make(map[utils.MyULID]*conf.FilesystemSourceConfig)
	for i := range c.FSSource {
		s.confs[c.FSSource[i].ConfID] = &(c.FSSource[i])
	}
	s.confsMap = make(map[ulid.ULID]utils.MyULID)
	s.parserEnv = decoders.NewParsersEnv(c.Parsers, s.logger)
}

func MakeFilter(globstring string) (tail.FilterFunc, error) {
	g, err := glob.Compile(globstring)
	if err != nil {
		return nil, err
	}
	f := func(relname string) bool {
		return g.Match(relname)
	}
	return f, nil
}

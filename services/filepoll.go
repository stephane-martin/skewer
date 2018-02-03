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
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/utils"
)

func initPollingRegistry() {
	base.Once.Do(func() {
		base.InitRegistry()
	})
}

type FilePollingService struct {
	pool           *sync.Pool
	stasher        base.Stasher
	logger         log15.Logger
	confs          map[utils.MyULID](*conf.FilesystemSourceConfig)
	confsMap       map[ulid.ULID]utils.MyULID
	ParserConfigs  []conf.ParserConfig
	tailor         *tail.Tailor
	rawQueue       chan *model.RawFileMessage
	fatalErrorChan chan struct{}
	fatalOnce      *sync.Once
	registryOnce   sync.Once
	confined       bool
	wg             sync.WaitGroup
	nWatchedFiles  prometheus.GaugeFunc
	nWatchedDirs   prometheus.GaugeFunc
}

func NewFilePollingService(env *base.ProviderEnv) (base.Provider, error) {
	initPollingRegistry()
	s := FilePollingService{
		stasher:  env.Reporter,
		logger:   env.Logger.New("class", "filepoll"),
		confined: env.Confined,
		pool: &sync.Pool{
			New: func() interface{} {
				return &model.RawFileMessage{}
			},
		},
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
	s.rawQueue = make(chan *model.RawFileMessage)

	results := make(chan tail.FileLineID)
	errors := make(chan error)
	tailor, err := tail.NewTailor(results, errors)
	if err != nil {
		return infos, err
	}
	s.tailor = tailor

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
	go s.fetchLines(results)
	s.wg.Add(1)
	go s.fetchErrors(errors)
	cpus := runtime.NumCPU()
	for i := 0; i < cpus; i++ {
		s.wg.Add(1)
		go s.parse()
	}

	return infos, nil
}

func makeFLogger(logger log15.Logger, raw *model.RawFileMessage) log15.Logger {
	return logger.New(
		"protocol", "filepoll",
		"format", raw.Format,
		"encoding", raw.Encoding,
		"filename", raw.Filename,
	)
}

func (s *FilePollingService) parseOne(raw *model.RawFileMessage, env *base.ParsersEnv, gen *utils.Generator) {

	decoder := utils.SelectDecoder(raw.Encoding)
	parser, err := env.GetParser(raw.Format)

	if parser == nil || err != nil {
		makeFLogger(s.logger, raw).Error("Unknown parser")
		return
	}

	syslogMsg, err := parser(raw.Line, decoder)
	if err != nil {
		base.ParsingErrorCounter.WithLabelValues("filepoll", raw.Hostname, raw.Format).Inc()
		makeFLogger(s.logger, raw).Info("Parsing error", "error", err)
		return
	}
	if syslogMsg == nil {
		makeFLogger(s.logger, raw).Debug("Empty message")
		return
	}
	syslogMsg.SetProperty("skewer", "client", raw.Hostname)
	syslogMsg.SetProperty("skewer", "filename", raw.Filename)
	syslogMsg.SetProperty("skewer", "directory", raw.Directory)

	full := model.FullFactoryFrom(syslogMsg)
	full.Uid = gen.Uid()
	full.ConfId = raw.ConfID

	fatal, nonfatal := s.stasher.Stash(full)

	if fatal != nil {
		makeFLogger(s.logger, raw).Error("Fatal error stashing filepoll message", "error", fatal)
		s.dofatal()
	} else if nonfatal != nil {
		makeFLogger(s.logger, raw).Warn("Non-fatal error stashing filepoll message", "error", nonfatal)
	}
	model.FullFree(full)

}

func (s *FilePollingService) parse() {
	defer s.wg.Done()

	env := base.NewParsersEnv(s.ParserConfigs, s.logger)
	gen := utils.NewGenerator()
	var raw *model.RawFileMessage
	for raw = range s.rawQueue {
		if raw == nil {
			return
		}
		s.parseOne(raw, env, gen)
		s.pool.Put(raw)
	}
}

func (s *FilePollingService) fetchErrors(errors chan error) {
	defer s.wg.Done()
	var err error
	for err = range errors {
		switch e := err.(type) {
		case *tail.FileErrorID:
			s.logger.Warn("Error watching file", "error", e.Err, "filename", e.Filename)
		case *tail.FileError:
			s.logger.Warn("Error watching file", "error", e.Err, "filename", e.Filename)
		default:
			s.logger.Warn("Error watching file", "error", err)
		}
	}
}

func (s *FilePollingService) fetchLines(results chan tail.FileLineID) {
	defer func() {
		close(s.rawQueue)
		s.wg.Done()
	}()

	var result tail.FileLineID
	var config *conf.FilesystemSourceConfig
	var raw *model.RawFileMessage

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	for result = range results {
		config = s.confs[s.confsMap[result.Uid]]
		raw = s.pool.Get().(*model.RawFileMessage)
		raw.Hostname = hostname
		raw.Encoding = config.Encoding
		raw.Format = config.Format
		raw.Directory = config.BaseDirectory
		raw.Glob = config.Glob
		raw.Filename = result.Filename
		if s.confined && len(raw.Filename) >= 13 {
			raw.Filename = raw.Filename[13:] // /tmp/polldirs/...
		}
		raw.Line = result.Line
		raw.ConfID = config.ConfID
		base.IncomingMsgsCounter.WithLabelValues("filepoll", hostname, "0", config.BaseDirectory)
		s.rawQueue <- raw
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
	s.ParserConfigs = c.Parsers
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

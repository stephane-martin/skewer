package services

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/inconshreveable/log15"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/accounting"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
)

func initAccountingRegistry() {
	base.Once.Do(func() {
		base.InitRegistry()
	})
}

type AccountingService struct {
	stasher        *base.Reporter
	logger         log15.Logger
	wgroup         sync.WaitGroup
	Conf           conf.AccountingSourceConfig
	stop           context.CancelFunc
	fatalErrorChan chan struct{}
	fatalOnce      *sync.Once
	confined       bool
}

func NewAccountingService(env *base.ProviderEnv) (base.Provider, error) {
	initAccountingRegistry()
	s := AccountingService{
		stasher:  env.Reporter,
		logger:   env.Logger.New("class", "accounting"),
		confined: env.Confined,
	}
	return &s, nil
}

func (s *AccountingService) Type() base.Types {
	return base.Accounting
}

func (s *AccountingService) Gather() ([]*dto.MetricFamily, error) {
	return base.Registry.Gather()
}

func readFileUntilEnd(f *os.File, size int) error {
	// read the acct file until the end
	buf := make([]byte, accounting.Ssize)
	reader := bufio.NewReader(f)
	for {
		_, err := io.ReadFull(reader, buf)
		if eerrors.HasFileClosed(err) {
			// we are at the end of the file
			return nil
		} else if err != nil {
			return eerrors.Fatal(eerrors.Wrap(err, "Failed to read the accounting file"))
		}
	}
}

func (s *AccountingService) makeMessage(buf []byte, tick int64, hostname string, gen *utils.Generator) *model.FullMessage {
	acct := accounting.MakeAcct(buf, tick)
	props := acct.Properties()
	fields := model.Factory()
	fields.AppName = "accounting"
	fields.Facility = model.Fuser
	fields.Severity = model.Sinfo
	fields.SetPriority()
	fields.HostName = hostname
	fields.MsgId = ""
	fields.ProcId = props["pid"]
	fields.Structured = ""
	fields.TimeReportedNum = acct.Btime.UnixNano()
	fields.TimeGeneratedNum = time.Now().UnixNano()
	fields.Version = 1
	fields.Message = acct.Marshal()
	fields.ClearDomain("accounting")
	fields.Properties.Map["accounting"].Map = acct.Properties()
	fields.SetProperty("skewer", "client", hostname)

	full := model.FullFactoryFrom(fields)
	full.Uid = gen.Uid()
	full.ConfId = s.Conf.ConfID
	return full
}

var ErrTruncated = eerrors.New("file has been truncated")

func (s *AccountingService) readFile(ctx context.Context, f *os.File, tick int64, hostname string, size int) error {
	var offset int64
	var fsize int64
	var infos os.FileInfo
	var full *model.FullMessage
	buf := make([]byte, accounting.Ssize)
	gen := utils.NewGenerator()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		_, err := io.ReadFull(f, buf)
		if eerrors.HasFileClosed(err) {
			// check if file has been truncated
			offset, err = f.Seek(0, 1)
			if err != nil {
				return err
			}
			infos, err = f.Stat()
			if err != nil {
				return err
			}
			fsize = infos.Size()
			if offset > fsize {
				return ErrTruncated
			}
			// we reached the end of the file
			return nil
		} else if err != nil {
			return err
		}
		full = s.makeMessage(buf, tick, hostname, gen)
		err = s.stasher.Stash(full)
		model.FullFree(full)
		if eerrors.IsFatal(err) {
			return err
		}
		if err != nil {
			s.logger.Warn("Non-fatal error stashing accounting message", "error", err)
			continue
		}
		base.IncomingMsgsCounter.WithLabelValues("accounting", hostname, "", "").Inc()
	}
}

func (s *AccountingService) doStart(ctx context.Context, watcher *fsnotify.Watcher, hostname string, f *os.File, tick int64) error {

Read:
	// fetch content from the acct file
	for {
		err := s.readFile(ctx, f, tick, hostname, accounting.Ssize)
		if err == ErrTruncated {
			s.logger.Info("Accounting file has been truncated")
			_, err = f.Seek(0, 0)
			if err != nil {
				return eerrors.Fatal(eerrors.Wrap(err, "Failed to seek to the beginning of the accounting file"))
			}
			continue Read
		} else if err != nil {
			return eerrors.Fatal(eerrors.Wrap(err, "Error reading the accounting file"))
		}
		select {
		case <-ctx.Done():
			return nil
		default:
		}

	WaitWrite:
		for {
			select {
			case err := <-watcher.Errors:
				s.logger.Warn("Watcher error", "error", err)
			case ev := <-watcher.Events:
				switch ev.Op {
				case fsnotify.Write:
					break WaitWrite
				case fsnotify.Rename:
					// accounting file rotation
					s.logger.Info("Accounting file has been renamed (rotation?)", "notifypath", ev.Name)
					time.Sleep(3 * time.Second)
					f2, err := os.Open(s.Conf.Path)
					if err != nil {
						return eerrors.Fatal(eerrors.Wrap(err, "Failed to reopen the accounting file"))
					}
					s.logger.Info("Accounting file has been reopened", "path", s.Conf.Path)
					err = s.doStart(ctx, watcher, hostname, f2, tick)
					f2.Close()
					return err
				case fsnotify.Remove:
					return eerrors.Fatal(eerrors.Wrap(err, "Accounting file has disappeared"))
				default:
				}
			case <-ctx.Done():
				return nil
			}
		}

	}

}

func (s *AccountingService) FatalError() chan struct{} {
	return s.fatalErrorChan
}

func (s *AccountingService) dofatal() {
	s.fatalOnce.Do(func() { close(s.fatalErrorChan) })
}

func (s *AccountingService) Start() (infos []model.ListenerInfo, err error) {
	var ctx context.Context
	infos = []model.ListenerInfo{}
	ctx, s.stop = context.WithCancel(context.Background())
	s.fatalErrorChan = make(chan struct{})
	s.fatalOnce = &sync.Once{}
	tick := accounting.Tick()

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	acctFilename := s.Conf.Path
	if s.confined {
		acctFilename = filepath.Join("/tmp", "acct", acctFilename)
	}

	f, err := os.Open(acctFilename)
	if err != nil {
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}

	s.wgroup.Add(1)
	go func() {
		defer func() {
			watcher.Close()
			f.Close()
			s.wgroup.Done()
		}()
		err := readFileUntilEnd(f, accounting.Ssize)
		if err != nil {
			s.logger.Error("Error reading the accounting file for the first time", "error", err)
			s.dofatal()
			return
		}
		s.logger.Debug("Finished going through accounting file")
		err = watcher.Add(s.Conf.Path)
		if err != nil {
			s.logger.Error("Failed to watch the accounting file", "error", err)
			s.dofatal()
			return
		}
		err = s.doStart(ctx, watcher, hostname, f, tick)
		if err != nil {
			s.logger.Error(err.Error())
			s.dofatal()
		}
	}()
	return infos, nil
}

func (s *AccountingService) Stop() {
	if s.stop != nil {
		s.stop()
	}
	s.wgroup.Wait()
}

func (s *AccountingService) Shutdown() {
	s.Stop()
}

func (s *AccountingService) SetConf(c conf.BaseConfig) {
	s.Conf = c.Accounting
}

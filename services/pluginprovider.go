package services

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/inconshreveable/log15"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/sys"
	"github.com/stephane-martin/skewer/utils"
)

var stdoutLock sync.Mutex

var SUCC []byte = []byte("SUCCESS")

func W(header string, message []byte) (err error) {
	stdoutLock.Lock()
	err = utils.W(os.Stdout, header, message)
	stdoutLock.Unlock()
	return err
}

type Reporter struct {
	name   string
	logger log15.Logger
}

func (s *Reporter) Stash(m *model.TcpUdpParsedMessage) (fatal error, nonfatal error) {
	// when the plugin *produces* a syslog message, write it to stdout
	b, err := m.MarshalMsg(nil)
	if err == nil {
		err = W("syslog", b)
		if err != nil {
			s.logger.Crit("Could not write message to upstream. There was message loss", "error", err)
			return err, nil
		} else {
			return nil, nil
		}
	} else {
		// should not happen
		s.logger.Warn("A syslog message could not be serialized", "type", s.name, "error", err)
		return nil, err
	}
}

func (s *Reporter) Report(infos []model.ListenerInfo) error {
	b, err := json.Marshal(infos)
	if err != nil {
		return err
	}
	return W("infos", b)
}

func Launch(typ NetworkServiceType, test bool, binderClient *sys.BinderClient, logger log15.Logger, pipe *os.File) error {
	generator := utils.Generator(context.Background(), logger)

	var command string
	var args string
	name := ReverseNetworkServiceMap[typ]
	hasConf := false

	reporter := &Reporter{name: name, logger: logger}
	svc := Factory(typ, reporter, generator, binderClient, logger, pipe)
	if svc == nil {
		err := fmt.Errorf("The Service Factory returned 'nil' for plugin '%s'", name)
		W("starterror", []byte(err.Error()))
		return err
	}

	var fatalChan chan struct{}

	var globalConf conf.BaseConfig
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(utils.PluginSplit)

	for scanner.Scan() {
		select {
		case <-fatalChan:
			svc.Shutdown()
			W("shutdown", []byte("fatal"))
			return fmt.Errorf("Store fatal error in plugin '%s'", name)
		default:
		}

		parts := strings.SplitN(scanner.Text(), " ", 2)
		command = parts[0]
		switch command {
		case "start":
			if !hasConf {
				err := fmt.Errorf("Configuration was not provided to plugin '%s' before start", name)
				W("syslogconferror", []byte(err.Error()))
				return err
			}
			infos, err := ConfigureAndStartService(svc, globalConf, test)
			if err != nil {
				W("starterror", []byte(err.Error()))
				return err
			} else if len(infos) == 0 && (typ == TCP || typ == UDP) {
				// only TCP and UDP directly report info about their effective listening ports
				svc.Stop()
				W("nolistenererror", []byte("plugin is inactive"))
			} else if typ == TCP {
				infosb, _ := json.Marshal(infos)
				W("started", infosb)
				reporter.Report(infos)
			} else {
				infosb, _ := json.Marshal(infos)
				W("started", infosb)
			}
			if typ == Store {
				// monitor the Store fatal errors
				fatalChan = svc.(StoreService).Errors()
			}
		case "stop":
			svc.Stop()
			W("stopped", SUCC)
			// here we *do not return*. So the plugin process continues to live
			// and to listen for subsequent control commands
		case "shutdown":
			svc.Shutdown()
			W("shutdown", SUCC)
			// at the end of shutdown command, we *return*. So the plugin
			// process stops right now.
			return nil
		case "conf":
			args = parts[1]
			c := conf.BaseConfig{}
			err := json.Unmarshal([]byte(args), &c)
			if err == nil {
				globalConf = c
				hasConf = true
			} else {
				W("conferror", []byte(err.Error()))
				return err
			}
		case "gathermetrics":
			empty := []*dto.MetricFamily{}
			families, err := svc.Gather()
			if err != nil {
				logger.Warn("Error gathering metrics", "type", name, "error", err)
				families = empty
			}
			familiesb, err := json.Marshal(families)
			if err != nil {
				logger.Warn("Error marshaling metrics", "type", name, "error", err)
				familiesb, _ = json.Marshal(empty)
			}
			err = W("metrics", familiesb)
			if err != nil {
				logger.Crit("Could not write metrics to upstream", "type", name, "error", err)
				return err
			}
		case "storemessage":
			// the service is asked to store a syslog message
			if stasher, ok := svc.(model.Stasher); ok {
				m := &model.TcpUdpParsedMessage{}
				_, err := m.UnmarshalMsg([]byte(parts[1]))
				if err == nil {
					fatal, nonfatal := stasher.Stash(m)
					if fatal != nil {
						logger.Crit("storemessage fatal error", "error", fatal)
						return fatal
					} else if nonfatal != nil {
						logger.Warn("storemessage error", "error", nonfatal)
					}

				} else {
					logger.Error("Error unmarshaling message", "type", name, "error", err)
				}
			} else {
				return fmt.Errorf("Plugin provider '%s' was asked to store a message but does not implement Stasher", name)
			}

		default:
			logger.Crit("Unknown command", "type", name, "command", command)
			return fmt.Errorf("Unknown command '%s' in plugin '%s'", command, name)
		}

	}
	e := scanner.Err()
	if e != nil {
		logger.Error("In plugin provider, scanning stdin met error", "error", e)
		return e
	}
	return nil
}

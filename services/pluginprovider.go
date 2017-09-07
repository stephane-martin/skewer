package services

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/sys"
	"github.com/stephane-martin/skewer/utils"
)

type Stasher struct {
	typ    NetworkServiceType
	logger log15.Logger
}

func (s *Stasher) Stash(m *model.TcpUdpParsedMessage) {
	// when the plugin *produces* a syslog message, write it to stdout
	b, err := m.MarshalMsg(nil)
	if err == nil {
		utils.W(os.Stdout, "syslog", b)
	} else {
		// should not happen
		s.logger.Warn("A syslog message could not be serialized", "type", s.typ)
	}
}

func Launch(typ NetworkServiceType, test bool, binderClient *sys.BinderClient, logger log15.Logger) error {
	generator := utils.Generator(context.Background(), logger)

	var command string
	var args string

	stasher := Stasher{typ: typ, logger: logger}
	svc := Factory(typ, &stasher, generator, binderClient, logger)
	if svc == nil {
		err := fmt.Errorf("The Service Factory returned nil!")
		utils.W(os.Stdout, "starterror", []byte(err.Error()))
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
			svc.WaitClosed()
			utils.W(os.Stdout, "shutdown", []byte("fatal"))
			return fmt.Errorf("Store fatal error")
		default:
		}

		parts := strings.SplitN(scanner.Text(), " ", 2)
		command = parts[0]
		switch command {
		case "start":
			// TODO
			//			if provider.syslogConfs == nil || provider.parserConfs == nil || provider.kafkaConf == nil || provider.auditConf == nil {
			//				err := fmt.Errorf("syslog conf or parser conf was not provided to plugin")
			//				utils.W(os.Stdout, "syslogconferror", []byte(err.Error()))
			//				return err
			//} else {
			infos, err := ConfigureAndStartService(svc, globalConf, test)
			if err != nil {
				utils.W(os.Stdout, "starterror", []byte(err.Error()))
				return err
			} else if len(infos) == 0 && typ != RELP && typ != Journal && typ != Audit && typ != Store {
				// (RELP, Journal and audit never report info about listening ports)
				svc.Stop()
				utils.W(os.Stdout, "nolistenererror", []byte("plugin is inactive"))
			} else {
				infosb, _ := json.Marshal(infos)
				utils.W(os.Stdout, "started", infosb)
			}
			if typ == Store {
				// monitor for the Store fatal errors
				fatalChan = svc.(StoreService).Errors()
			}
		case "stop":
			svc.Stop()
			svc.WaitClosed()
			utils.W(os.Stdout, "stopped", []byte("success"))
			// at the end of the stop return, we *do not return*. So the
			// plugin process continues to listenn for subsequent commands
		case "shutdown":
			svc.Shutdown()
			svc.WaitClosed()
			utils.W(os.Stdout, "shutdown", []byte("success"))
			// at the end of shutdown command, we *return*. And the plugin process stops.
			return nil
		case "conf":
			args = parts[1]
			c := conf.BaseConfig{}
			err := json.Unmarshal([]byte(args), &c)
			if err == nil {
				globalConf = c
			} else {
				utils.W(os.Stdout, "conferror", []byte(err.Error()))
				return err
			}
		case "gathermetrics":
			families, err := svc.Gather()
			if err != nil {
				// TODO
			} else {
				familiesb, err := json.Marshal(families)
				if err != nil {
					// TODO
				} else {
					utils.W(os.Stdout, "metrics", familiesb)
				}
			}
		case "stash":
			// the service is asked to store a syslog message
			args = parts[1]
			m := &model.TcpUdpParsedMessage{}
			_, err := m.UnmarshalMsg([]byte(args))
			if err == nil {
				// does the service support stashing?
				if stasher, ok := svc.(model.Stasher); ok {
					stasher.Stash(m)
				} else {
					return fmt.Errorf("Plugin provider was asked to store a message but does not implement Stasher")
				}
			}
		default:
			return fmt.Errorf("Unknown command")
		}

	}
	e := scanner.Err()
	if e != nil {
		logger.Error("In plugin provider, scanning stdin met error", "error", e)
		return e
	}
	return nil
}

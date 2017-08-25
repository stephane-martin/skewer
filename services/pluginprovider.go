package services

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/sys"
	"github.com/stephane-martin/skewer/utils"
)

// NetworkPluginProvider implements the TCP service in a separated process
type NetworkPluginProvider struct {
	svc         NetworkService
	logger      log15.Logger
	syslogConfs []*conf.SyslogConfig
	parserConfs []conf.ParserConfig
	kafkaConf   *conf.KafkaConfig
	auditConf   *conf.AuditConfig
}

func (p *NetworkPluginProvider) Stash(m *model.TcpUdpParsedMessage) {
	b, err := m.MarshalMsg(nil)
	if err == nil {
		utils.W(os.Stdout, "syslog", b)
	} else {
		// should not happen
		p.logger.Warn("In plugin, a syslog message could not be serialized to JSON ?!")
	}
}

func (p *NetworkPluginProvider) Launch(typ string, test bool, binderClient *sys.BinderClient, logger log15.Logger) error {
	generator := utils.Generator(context.Background(), logger)
	p.logger = logger

	var scanner *bufio.Scanner
	var command string
	var args string
	var cancel context.CancelFunc

	scanner = bufio.NewScanner(os.Stdin)
	scanner.Split(utils.PluginSplit)

	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), " ", 2)
		command = parts[0]
		switch command {
		case "start":
			if p.syslogConfs == nil || p.parserConfs == nil || p.kafkaConf == nil || p.auditConf == nil {
				utils.W(os.Stdout, "syslogconferror", []byte("syslog conf or parser conf was not provided to plugin"))
				p.svc = nil
				// TODO: return
			} else {
				p.svc, cancel = Factory(typ, p, generator, binderClient, logger)
				if p.svc == nil {
					utils.W(os.Stdout, "starterror", []byte("NewNetworkService returned nil"))
				} else {
					p.svc.SetConf(p.syslogConfs, p.parserConfs)
					p.svc.SetKafkaConf(p.kafkaConf)
					p.svc.SetAuditConf(p.auditConf)
					infos, err := p.svc.Start(test)
					if err != nil {
						utils.W(os.Stdout, "starterror", []byte(err.Error()))
						p.svc = nil
					} else if len(infos) == 0 && typ != "skewer-relp" && typ != "skewer-journal" && typ != "skewer-audit" {
						// (RELP, Journal and audit never report info about listening ports)
						p.svc.Stop()
						p.svc = nil
						utils.W(os.Stdout, "nolistenererror", []byte("plugin is inactive"))
					} else {
						infosb, _ := json.Marshal(infos)
						utils.W(os.Stdout, "started", infosb)
					}
				}
			}
		case "stop":
			if p.svc != nil {
				p.svc.Stop()
				p.svc.WaitClosed()

			}
			utils.W(os.Stdout, "stopped", []byte("success"))
			// at the end of the stop return, we *do not return*. So the
			// plugin process continues to listenn for subsequent commands
		case "shutdown":
			if p.svc != nil {
				p.svc.Stop()
				p.svc.WaitClosed()
			}
			if cancel != nil {
				cancel()
				time.Sleep(400 * time.Millisecond) // give a chance for cleaning to be executed before plugin process ends
			}
			utils.W(os.Stdout, "shutdown", []byte("success"))
			// at the end of shutdown command, we *return*. And the plugin process stops.
			return nil
		case "syslogconf":
			args = parts[1]
			sc := []*conf.SyslogConfig{}
			err := json.Unmarshal([]byte(args), &sc)
			if err == nil {
				p.syslogConfs = sc
			} else {
				p.syslogConfs = nil
				utils.W(os.Stdout, "syslogconferror", []byte(err.Error()))
			}
		case "parserconf":
			args = parts[1]
			pc := []conf.ParserConfig{}
			err := json.Unmarshal([]byte(args), &pc)
			if err == nil {
				p.parserConfs = pc
			} else {
				p.parserConfs = nil
				utils.W(os.Stdout, "parserconferror", []byte(err.Error()))
			}
		case "kafkaconf":
			args = parts[1]
			kc := conf.KafkaConfig{}
			err := json.Unmarshal([]byte(args), &kc)
			if err == nil {
				p.kafkaConf = &kc
			} else {
				p.kafkaConf = nil
				utils.W(os.Stdout, "kafkaconferror", []byte(err.Error()))
			}
		case "auditconf":
			args = parts[1]
			ac := conf.AuditConfig{}
			err := json.Unmarshal([]byte(args), &ac)
			if err == nil {
				p.auditConf = &ac
			} else {
				p.auditConf = nil
				utils.W(os.Stdout, "auditconferror", []byte(err.Error()))
			}
		case "gathermetrics":
			families, err := p.svc.Gather()
			if err != nil {
				// TODO
			}
			familiesb, err := json.Marshal(families)
			if err == nil {
				utils.W(os.Stdout, "metrics", familiesb)
			} else {
				// TODO
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

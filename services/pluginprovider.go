package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/inconshreveable/log15"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/sys/binder"
	"github.com/stephane-martin/skewer/utils"
)

func Launch(typ NetworkServiceType, test bool, binderClient *binder.BinderClient, logger log15.Logger, pipe *os.File) error {
	generator := utils.Generator(context.Background(), logger)

	var command string
	name := ReverseNetworkServiceMap[typ]
	hasConf := false

	reporter := &base.Reporter{Name: name, Logger: logger, Pipe: pipe}
	svc := Factory(typ, reporter, generator, binderClient, logger, pipe)
	if svc == nil {
		err := fmt.Errorf("The Service Factory returned 'nil' for plugin '%s'", name)
		base.W("starterror", []byte(err.Error()))
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
			base.W("shutdown", []byte("fatal"))
			return fmt.Errorf("Store fatal error in plugin '%s'", name)
		default:
		}

		parts := bytes.SplitN(scanner.Bytes(), SP, 2)
		command = string(parts[0])
		switch command {
		case "start":
			if !hasConf {
				err := fmt.Errorf("Configuration was not provided to plugin '%s' before start", name)
				base.W("syslogconferror", []byte(err.Error()))
				return err
			}
			infos, err := ConfigureAndStartService(svc, globalConf, test)
			if err != nil {
				base.W("starterror", []byte(err.Error()))
				return err
			} else if len(infos) == 0 && (typ == TCP || typ == UDP) {
				// only TCP and UDP directly report info about their effective listening ports
				svc.Stop()
				base.W("nolistenererror", []byte("plugin is inactive"))
			} else if typ == TCP {
				infosb, _ := json.Marshal(infos)
				base.W("started", infosb)
				reporter.Report(infos)
			} else {
				infosb, _ := json.Marshal(infos)
				base.W("started", infosb)
			}
			if typ == Store {
				// monitor the Store fatal errors
				fatalChan = svc.(StoreService).Errors()
			}
		case "stop":
			svc.Stop()
			base.W("stopped", base.SUCC)
			// here we *do not return*. So the plugin process continues to live
			// and to listen for subsequent control commands
		case "shutdown":
			logger.Debug("provider is asked to stop", "type", name)
			svc.Shutdown()
			base.W("shutdown", base.SUCC)
			// at the end of shutdown command, we *return*. So the plugin
			// process stops right now.
			return nil
		case "conf":
			c := conf.BaseConfig{}
			err := json.Unmarshal(parts[1], &c)
			if err == nil {
				globalConf = c
				hasConf = true
			} else {
				base.W("conferror", []byte(err.Error()))
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
			err = base.W("metrics", familiesb)
			if err != nil {
				logger.Crit("Could not write metrics to upstream", "type", name, "error", err)
				return err
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

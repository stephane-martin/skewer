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
	"github.com/stephane-martin/skewer/sys/kring"
	"github.com/stephane-martin/skewer/utils"
)

func Launch(typ NetworkServiceType, test bool, r kring.Ring, binderClient *binder.BinderClient, logger log15.Logger, pipe *os.File) error {
	if r == nil {
		return fmt.Errorf("No ring")
	}
	generator := utils.Generator(context.Background(), logger)

	var command string
	name := ReverseNetworkServiceMap[typ]
	hasConf := false

	reporter := base.NewReporter(name, logger, pipe)
	defer reporter.Stop()

	svc := Factory(typ, r, reporter, generator, binderClient, logger, pipe)
	if svc == nil {
		err := fmt.Errorf("The Service Factory returned 'nil' for plugin '%s'", name)
		base.Wout(STARTERROR, []byte(err.Error()), nil)
		return err
	}

	var fatalChan chan struct{}

	var globalConf conf.BaseConfig

	signpubkey, err := r.GetSignaturePubkey()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(utils.MakeSignSplit(signpubkey))
	//scanner.Split(utils.PluginSplit)

	for scanner.Scan() {
		select {
		case <-fatalChan:
			svc.Shutdown()
			base.Wout(SHUTDOWN, []byte("fatal"), nil)
			return fmt.Errorf("Store fatal error in plugin '%s'", name)
		default:
		}

		parts := bytes.SplitN(scanner.Bytes(), space, 2)
		command = string(parts[0])
		switch command {
		case "start":
			if !hasConf {
				err := fmt.Errorf("Configuration was not provided to plugin '%s' before start", name)
				base.Wout([]byte("syslogconferror"), []byte(err.Error()), nil)
				return err
			}
			if globalConf.Main.EncryptIPC {
				logger.Debug("Encrypting messages from plugin", "type", name)
				secret, err := r.GetBoxSecret()
				if err != nil {
					base.Wout(STARTERROR, []byte(err.Error()), nil)
					return err
				}
				reporter.SetSecret(secret)
			} else {
				reporter.SetSecret(nil)
			}
			reporter.Start()
			infos, err := ConfigureAndStartService(svc, globalConf, test)
			if err != nil {
				base.Wout(STARTERROR, []byte(err.Error()), nil)
				return err
			} else if len(infos) == 0 && (typ == TCP || typ == UDP) {
				// only TCP and UDP directly report info about their effective listening ports
				svc.Stop()
				base.Wout([]byte("nolistenererror"), []byte("plugin is inactive"), nil)
			} else if typ == TCP {
				infosb, _ := json.Marshal(infos)
				base.Wout(STARTED, infosb, nil)
				reporter.Report(infos)
			} else {
				infosb, _ := json.Marshal(infos)
				base.Wout(STARTED, infosb, nil)
			}
			if typ == Store {
				// monitor the Store fatal errors
				fatalChan = svc.(StoreService).Errors()
			}
		case "stop":
			svc.Stop()
			base.Wout(STOPPED, base.SUCC, nil)
			// here we *do not return*. So the plugin process continues to live
			// and to listen for subsequent control commands
		case "shutdown":
			logger.Debug("provider is asked to stop", "type", name)
			svc.Shutdown()
			base.Wout(SHUTDOWN, base.SUCC, nil)
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
				base.Wout(CONFERROR, []byte(err.Error()), nil)
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
			err = base.Wout(METRICS, familiesb, nil)
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

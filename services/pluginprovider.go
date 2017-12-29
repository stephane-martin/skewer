package services

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/inconshreveable/log15"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/sys/binder"
	"github.com/stephane-martin/skewer/sys/kring"
	"github.com/stephane-martin/skewer/utils"
)

var stdoutMu sync.Mutex
var stdoutWriter *utils.EncryptWriter

func init() {
	stdoutWriter = utils.NewEncryptWriter(os.Stdout, nil)
}

func Wout(header []byte, msg []byte) (err error) {
	stdoutMu.Lock()
	err = stdoutWriter.WriteWithHeader(header, msg)
	stdoutMu.Unlock()
	return err
}

func Launch(typ Types, confined bool, profile bool, ring kring.Ring, binderClt *binder.BinderClientImpl, l log15.Logger, pipe *os.File) error {
	if ring == nil {
		return fmt.Errorf("No ring")
	}

	var command string
	name := Types2Names[typ]
	hasConf := false

	var reporter base.Reporter
	if typ != Store && typ != Configuration {
		if pipe == nil {
			return fmt.Errorf("Plugin '%s' has a nil pipe", name)
		}
		reporter = base.NewReporter(name, l, pipe)
		defer reporter.Stop() // will close the pipe
	}

	svc := ProviderFactory(typ, confined, profile, ring, reporter, binderClt, l, pipe)
	if svc == nil {
		err := fmt.Errorf("The Service Factory returned 'nil' for plugin '%s'", name)
		_ = Wout(STARTERROR, []byte(err.Error()))
		return err
	}

	var fatalChan chan struct{}

	var globalConf conf.BaseConfig

	signpubkey, err := ring.GetSignaturePubkey()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(utils.MakeSignSplit(signpubkey))

	for scanner.Scan() {
		select {
		case <-fatalChan:
			svc.Shutdown()
			_ = Wout(SHUTDOWN, []byte("fatal"))
			return fmt.Errorf("Store fatal error in plugin '%s'", name)
		default:
		}

		parts := bytes.SplitN(scanner.Bytes(), space, 2)
		command = string(parts[0])
		switch command {
		case "start":
			if !hasConf {
				err := fmt.Errorf("Configuration was not provided to plugin '%s' before start", name)
				_ = Wout([]byte("syslogconferror"), []byte(err.Error()))
				return err
			}
			if reporter != nil {
				if globalConf.Main.EncryptIPC {
					l.Debug("Encrypting messages from plugin", "type", name)
					secret, err := ring.GetBoxSecret()
					if err != nil {
						_ = Wout(STARTERROR, []byte(err.Error()))
						return err
					}
					reporter.SetSecret(secret)
				} else {
					reporter.SetSecret(nil)
				}
				reporter.Start()
			}
			infos, err := ConfigureAndStartService(svc, globalConf)
			if err != nil {
				_ = Wout(STARTERROR, []byte(err.Error()))
				return err
			} else if len(infos) == 0 && (typ == TCP || typ == UDP) {
				// only TCP and UDP directly report info about their effective listening ports
				svc.Stop()
				err := Wout([]byte("nolistenererror"), []byte("plugin is inactive"))
				if err != nil {
					return err
				}
			} else if typ == TCP {
				infosb, _ := json.Marshal(infos)
				err := Wout(STARTED, infosb)
				if err != nil {
					return err
				}
				err = reporter.Report(infos)
				if err != nil {
					return err
				}
			} else {
				infosb, _ := json.Marshal(infos)
				err := Wout(STARTED, infosb)
				if err != nil {
					return err
				}
			}
			fatalChan = svc.FatalError()
		case "stop":
			svc.Stop()
			err := Wout(STOPPED, base.SUCC)
			if err != nil {
				return err
			}
			// here we *do not return*. So the plugin process continues to live
			// and to listen for subsequent control commands
		case "shutdown":
			l.Debug("provider is asked to stop", "type", name)
			svc.Shutdown()
			_ = Wout(SHUTDOWN, base.SUCC)
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
				_ = Wout(CONFERROR, []byte(err.Error()))
				return err
			}
		case "gathermetrics":
			empty := []*dto.MetricFamily{}
			families, err := svc.Gather()
			if err != nil {
				l.Warn("Error gathering metrics", "type", name, "error", err)
				families = empty
			}
			familiesb, err := json.Marshal(families)
			if err != nil {
				l.Warn("Error marshaling metrics", "type", name, "error", err)
				familiesb, _ = json.Marshal(empty)
			}
			err = Wout(METRICS, familiesb)
			if err != nil {
				l.Crit("Could not write metrics to upstream", "type", name, "error", err)
				return err
			}
		default:
			l.Crit("Unknown command", "type", name, "command", command)
			return fmt.Errorf("Unknown command '%s' in plugin '%s'", command, name)
		}

	}
	e := scanner.Err()
	if e != nil {
		l.Error("In plugin provider, scanning stdin met error", "error", e)
		return e
	}
	return nil
}

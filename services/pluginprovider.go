package services

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/services/base"
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

func Launch(typ base.Types, opts ...ProviderOpt) error {
	env := &base.ProviderEnv{}
	for _, opt := range opts {
		opt(env)
	}

	var command string
	name := base.Types2Names[typ]
	hasConf := false

	if typ != base.Store && typ != base.Configuration {
		if env.Pipe == nil {
			return fmt.Errorf("Plugin '%s' has a nil pipe", name)
		}
		SetReporter(base.NewReporter(name, env.Logger, env.Pipe))(env)
		defer env.Reporter.Stop() // will close the pipe
	}

	svc, err := ProviderFactory(typ, env)
	if err != nil {
		err = fmt.Errorf("The Service Factory returned an error for plugin '%s': %s", name, err.Error())
		_ = Wout(STARTERROR, []byte(err.Error()))
		return err
	}
	if svc == nil {
		err := fmt.Errorf("The Service Factory returned 'nil' for plugin '%s'", name)
		_ = Wout(STARTERROR, []byte(err.Error()))
		return err
	}

	var fatalChan chan struct{}

	var globalConf conf.BaseConfig

	signpubkey, err := env.Ring.GetSignaturePubkey()
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
			if env.Reporter != nil {
				if globalConf.Main.EncryptIPC {
					env.Logger.Debug("Encrypting messages from plugin", "type", name)
					secret, err := env.Ring.GetBoxSecret()
					if err != nil {
						_ = Wout(STARTERROR, []byte(err.Error()))
						return err
					}
					env.Reporter.SetSecret(secret)
				} else {
					env.Reporter.SetSecret(nil)
				}
				env.Reporter.Start()
			}
			infos, err := ConfigureAndStartService(svc, globalConf)
			if err != nil {
				_ = Wout(STARTERROR, []byte(err.Error()))
				return err
			} else if len(infos) == 0 && (typ == base.TCP || typ == base.UDP) {
				// only TCP and UDP directly report info about their effective listening ports
				svc.Stop()
				err := Wout([]byte("nolistenererror"), []byte("plugin is inactive"))
				if err != nil {
					return err
				}
			} else if typ == base.TCP {
				infosb, _ := json.Marshal(infos)
				err := Wout(STARTED, infosb)
				if err != nil {
					return err
				}
				err = env.Reporter.Report(infos)
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
			env.Logger.Debug("provider is asked to stop", "type", name)
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
				env.Logger.Warn("Error gathering metrics", "type", name, "error", err)
				families = empty
			}
			familiesb, err := json.Marshal(families)
			if err != nil {
				env.Logger.Warn("Error marshaling metrics", "type", name, "error", err)
				familiesb, _ = json.Marshal(empty)
			}
			err = Wout(METRICS, familiesb)
			if err != nil {
				env.Logger.Crit("Could not write metrics to upstream", "type", name, "error", err)
				return err
			}
		default:
			env.Logger.Crit("Unknown command", "type", name, "command", command)
			return fmt.Errorf("Unknown command '%s' in plugin '%s'", command, name)
		}

	}
	e := scanner.Err()
	if e != nil {
		env.Logger.Error("In plugin provider, scanning stdin met error", "error", e)
		return e
	}
	return nil
}

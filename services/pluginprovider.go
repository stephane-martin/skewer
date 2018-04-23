package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"sync"

	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
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
	return eerrors.Wrap(err, "error writing to stdout of plugin provider")
}

func Launch(ctx context.Context, typ base.Types, opts ...ProviderOpt) (err error) {
	// TODO: wrap all errors correctly
	name := base.Types2Names[typ]
	defer func() {
		if e := eerrors.Err(recover()); e != nil {
			err = eerrors.Wrapf(err, "Scanner panicked in plugin provider '%s'", name)
		}
	}()

	env := &base.ProviderEnv{}
	for _, opt := range opts {
		opt(env)
	}

	var globalConf conf.BaseConfig
	fatalctx, dofatal := context.WithCancel(ctx)
	var command string
	hasConf := false

	if typ != base.Store && typ != base.Configuration {
		if env.Pipe == nil {
			return eerrors.Errorf("Plugin '%s' has a nil pipe", name)
		}
		SetReporter(base.NewReporter(name, env.Logger, env.Pipe))(env)
		defer env.Reporter.Stop() // will close the pipe
	}

	svc, err := ProviderFactory(typ, env)
	if err != nil {
		err = eerrors.Wrapf(err, "The Service Factory returned an error for plugin '%s': %s", name)
		_ = Wout(STARTERROR, []byte(err.Error()))
		return err
	}
	if svc == nil {
		err := eerrors.Errorf("The Service Factory returned 'nil' for plugin '%s'", name)
		_ = Wout(STARTERROR, []byte(err.Error()))
		return err
	}

	signpubkey, err := env.Ring.GetSignaturePubkey()
	if err != nil {
		err = eerrors.Wrap(err, "Can't get the signature key")
		_ = Wout(STARTERROR, []byte(err.Error()))
		return err
	}

	scanner := utils.NewWrappedScanner(fatalctx, bufio.NewScanner(os.Stdin))
	scanner.Split(utils.MakeSignSplit(signpubkey))

	for scanner.Scan() {
		parts := bytes.SplitN(scanner.Bytes(), space, 2)
		command = string(parts[0])
		switch command {
		case "start":
			if !hasConf {
				err := eerrors.Errorf("Configuration was not provided to plugin '%s' before start", name)
				_ = Wout([]byte("syslogconferror"), []byte(err.Error()))
				return err
			}
			if env.Reporter != nil {
				if globalConf.Main.EncryptIPC {
					env.Logger.Debug("Encrypting messages from plugin", "type", name)
					secret, err := env.Ring.GetBoxSecret()
					if err != nil {
						err = eerrors.Wrap(err, "Can't get box secret")
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
				err = eerrors.Wrapf(err, "Can't configure service '%s'", name)
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
			go func() {
				<-svc.FatalError()
				dofatal()
			}()
		case "stop":
			svc.Stop()
			err = Wout(STOPPED, base.SUCC)
			if err != nil {
				return eerrors.Wrap(err, "Error reporting 'stopped' to the controller")
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
			err = json.Unmarshal(parts[1], &c)
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
				return eerrors.Wrap(err, "Can not write metrics to the controller")
			}
		default:
			env.Logger.Crit("Unknown command", "type", name, "command", command)
			return eerrors.Errorf("Unknown command '%s' received by plugin '%s'", command, name)
		}

	}
	select {
	case <-ctx.Done():
		svc.Shutdown()
		err = eerrors.Errorf("SIGTERM received by plugin '%s'", name)
		_ = Wout(SHUTDOWN, []byte(err.Error()))
		return nil
	default:
	}
	select {
	case <-fatalctx.Done():
		svc.Shutdown()
		err = eerrors.Errorf("Fatal error in plugin '%s'", name)
		_ = Wout(SHUTDOWN, []byte(err.Error()))
		return err
	default:
	}
	err = scanner.Err()
	if err != nil {
		err = eerrors.Wrapf(err, "Error scanning stdin of plugin '%s'", name)
		svc.Shutdown()
		_ = Wout(SHUTDOWN, []byte(err.Error()))
		return err
	}

	return nil
}

package services

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/awnumar/memguard"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/consul"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/sys/capabilities"
	"github.com/stephane-martin/skewer/sys/kring"
	"github.com/stephane-martin/skewer/sys/namespaces"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
)

var confStdoutMu sync.Mutex
var confStdoutWriter *utils.EncryptWriter

func WConf(header []byte, message []byte) (err error) {
	confStdoutMu.Lock()
	err = confStdoutWriter.WriteWithHeader(header, message)
	confStdoutMu.Unlock()
	return eerrors.Wrap(err, "child failed to write to its stdout pipe")
}

type ConfigurationService struct {
	output       chan *conf.BaseConfig
	params       consul.ConnParams
	stdin        io.WriteCloser
	logger       log15.Logger
	stdinMu      *sync.Mutex
	confdir      string
	loggerHandle uintptr
	signKey      *memguard.LockedBuffer
	boxsec       *memguard.LockedBuffer
	stdinWriter  *utils.SigWriter
}

func NewConfigurationService(ring kring.Ring, signKey *memguard.LockedBuffer, childLoggerHandle uintptr, l log15.Logger) (*ConfigurationService, error) {
	c := ConfigurationService{
		loggerHandle: childLoggerHandle,
		logger:       l,
		signKey:      signKey,
		stdinMu:      &sync.Mutex{},
	}
	boxsec, err := ring.GetBoxSecret()
	if err != nil {
		return nil, eerrors.Wrap(err, "Can't get the box secret")
	}
	c.boxsec = boxsec
	return &c, nil
}

func (c *ConfigurationService) Type() base.Types {
	return base.Configuration
}

func (c *ConfigurationService) W(header []byte, message []byte) (err error) {
	c.stdinMu.Lock()
	defer c.stdinMu.Unlock()
	if c.stdinWriter != nil {
		err = eerrors.Wrap(c.stdinWriter.WriteWithHeader(header, message), "Error writing to the configuration stdin pipe")
	} else {
		err = eerrors.New("stdin is nil")
	}
	return err
}

func (c *ConfigurationService) SetConfDir(cdir string) {
	c.confdir = cdir
}

func (c *ConfigurationService) SetConsulParams(params consul.ConnParams) {
	c.params = params
}

func (c *ConfigurationService) Stop() error {
	err := c.W([]byte("stop"), utils.NOW)
	if err != nil {
		return eerrors.Wrap(err, "Error asking the configuration plugin to stop")
	}
	for range c.output {
	}
	return nil
}

func (c *ConfigurationService) Reload() error {
	err := c.W([]byte("reload"), utils.NOW)
	if err != nil {
		return eerrors.Wrap(err, "Error asking the configuration plugin to reload")
	}
	return nil
}

func (c *ConfigurationService) Chan() chan *conf.BaseConfig {
	return c.output
}

func (c *ConfigurationService) Start(r kring.Ring) error {
	var err error
	var cmd *namespaces.PluginCmd
	c.output = make(chan *conf.BaseConfig)

	if capabilities.CapabilitiesSupported {
		cmd, err = namespaces.SetupCmd("confined-skewer-conf", r, namespaces.LoggerHandle(c.loggerHandle))
		if err != nil {
			close(c.output)
			return err
		}
		err = cmd.Namespaced().ConfPath(c.confdir).Start()
	}
	if err != nil {
		c.logger.Warn("Starting configuration service in user namespace has failed", "error", err)
	}
	if err != nil || !capabilities.CapabilitiesSupported {
		cmd, err = namespaces.SetupCmd("skewer-conf", r, namespaces.LoggerHandle(c.loggerHandle))
		if err != nil {
			close(c.output)
			return eerrors.Wrap(err, "error creating the execution environment for configuration child")
		}
		err = cmd.Cmd.Start()
	}
	if err != nil {
		_ = cmd.Stdin.Close()
		_ = cmd.Stdout.Close()
		close(c.output)
		return eerrors.Wrap(err, "error starting the configuration child")
	}
	c.stdin = cmd.Stdin
	c.stdinWriter = utils.NewSignatureWriter(cmd.Stdin, c.signKey)

	startedChan := make(chan error)

	go func() {
		kill := false
		once := &sync.Once{}
		defer func() {
			c.logger.Debug("Configuration service is stopping")

			once.Do(func() {
				startedChan <- eerrors.New("unexpected end of configuration service")
				close(startedChan)
			})

			if kill {
				c.logger.Warn("Killing configuration service")
				c.stdinMu.Lock()
				_ = cmd.Cmd.Process.Kill()
				c.stdinMu.Unlock()
			}

			errChan := make(chan error)
			go func() {
				errChan <- cmd.Cmd.Wait()
				close(errChan)
			}()

			select {
			case err = <-errChan:
			case <-time.After(3 * time.Second):
				c.logger.Warn("Timeout: killing configuration service")
				c.stdinMu.Lock()
				_ = cmd.Cmd.Process.Kill()
				c.stdinMu.Unlock()
				err = cmd.Cmd.Wait()
			}

			if err == nil {
				c.logger.Debug("Configuration process has exited without providing error")
			} else {
				c.logger.Error("Configuration process has ended with error", "error", err.Error())
				if e, ok := err.(*exec.ExitError); ok {
					status := e.Sys()
					if cstatus, ok := status.(syscall.WaitStatus); ok {
						c.logger.Error("Configuration process exit code", "code", cstatus.ExitStatus())
					}
				}
			}
			close(c.output)
		}()

		var command string
		scanner := bufio.NewScanner(cmd.Stdout)
		scanner.Split(utils.MakeDecryptSplit(c.boxsec))

		for {
			cont, err := utils.ScanRecover(scanner)
			if err != nil {
				c.logger.Crit("Scanner panicked in configuration controller", "error", err)
				kill = true
				return
			}
			if !cont {
				break
			}
			parts := strings.SplitN(scanner.Text(), " ", 2)
			command = parts[0]
			switch command {
			case "newconf":
				if len(parts) == 2 {
					newconf := conf.BaseConfig{}
					err := json.Unmarshal([]byte(parts[1]), &newconf)
					if err == nil {
						c.output <- &newconf
					} else {
						c.logger.Error("Error unmarshaling a new configuration received from child", "error", err)
						kill = true
						return
					}
				} else {
					c.logger.Error("Empty newconf message received from configuration child")
					kill = true
					return
				}
			case "started":
				c.logger.Debug("Configuration child has started")
				once.Do(func() { close(startedChan) })

			case "starterror":
				var err error
				if len(parts) == 2 {
					err = eerrors.Errorf("Configuration child failed to start: %s", parts[1])
				} else {
					err = eerrors.New("Unexpected empty start error from configuration child")
					kill = true
				}
				c.logger.Error(err.Error())
				once.Do(func() {
					startedChan <- err
					close(startedChan)
				})
				return
			case "reloaded":
				c.logger.Debug("Configuration child has been reloaded")
			default:
				err := eerrors.Errorf("Unknown command received from configuration child: %s", command)
				c.logger.Warn(err.Error())
				kill = true
				once.Do(func() {
					startedChan <- err
					close(startedChan)
				})
				return
			}
		}
		err := scanner.Err()
		if err != nil {
			c.logger.Error("Scanner error", "error", err)
		}

	}()

	cparams, _ := json.Marshal(c.params)
	//c.logger.Info("Consul params", "params", string(cparams))

	err = c.W([]byte("confdir"), []byte(c.confdir))
	if err == nil {
		err = c.W([]byte("consulparams"), cparams)
		if err == nil {
			err = c.W([]byte("start"), utils.NOW)
			if err == nil {
				err = <-startedChan
			}
		}
	}
	if err != nil {
		c.Stop()
		return eerrors.Wrap(err, "Error starting the configuration service")
	}
	return nil

}

func writeNewConf(ctx context.Context, updated chan *conf.BaseConfig, logger log15.Logger) {
	done := ctx.Done()
Loop:
	for {
		select {
		case <-done:
			return
		case newconf, more := <-updated:
			if !more {
				return
			}
			confb, err := json.Marshal(newconf)
			if err != nil {
				logger.Warn("Error serializing new configuration", "error", err)
				continue Loop
			}
			select {
			case <-done:
				// be extra sure not to use boxsec after we have been canceled
				return
			default:
				err = WConf([]byte("newconf"), confb)
				if err != nil {
					logger.Warn("Error sending new configuration", "error", err)
				}
			}
		}
	}
}

func start(confdir string, params consul.ConnParams, r kring.Ring, logger log15.Logger) (context.CancelFunc, error) {

	if len(confdir) == 0 {
		return nil, fmt.Errorf("configuration directory is empty")
	}
	ctx, cancel := context.WithCancel(context.Background())
	gconf, updated, err := conf.InitLoad(ctx, confdir, params, r, logger)
	if err == nil {
		confb, err := json.Marshal(gconf)
		if err == nil {
			err = utils.Chain(
				func() error { return WConf([]byte("started"), utils.NOW) },
				func() error { return WConf([]byte("newconf"), confb) },
			)
			if err != nil {
				return nil, err
			}
			go writeNewConf(ctx, updated, logger)
		} else {
			cancel()
			return nil, err
		}
	} else {
		cancel()
		return nil, err
	}
	return cancel, nil
}

func LaunchConfProvider(ctx context.Context, r kring.Ring, confined bool, logger log15.Logger) (err error) {
	if r == nil {
		return fmt.Errorf("no ring provided")
	}
	sigpubkey, err := r.GetSignaturePubkey()
	if err != nil {
		return err
	}
	boxsec, err := r.GetBoxSecret()
	if err != nil {
		return err
	}
	confStdoutWriter = utils.NewEncryptWriter(os.Stdout, boxsec)
	var confdir string
	var params consul.ConnParams

	scanner := utils.NewWrappedScanner(ctx, bufio.NewScanner(os.Stdin))
	scanner.Split(utils.MakeSignSplit(sigpubkey))
	var command string
	var cancel context.CancelFunc

	for {
		cont, err := utils.ScanRecover(scanner)
		if err != nil {
			return eerrors.Wrap(err, "Scanner panicked in configuration provider")
		}
		if !cont {
			break
		}
		parts := strings.SplitN(scanner.Text(), " ", 2)
		command = parts[0]

		switch command {
		case "start":
			var err error
			cancel, err = start(confdir, params, r, logger)
			if err != nil {
				_ = WConf([]byte("starterror"), []byte(err.Error()))
				return err
			}

		case "reload":
			newcancel, err := start(confdir, params, r, logger)
			if err == nil {
				if cancel != nil {
					cancel()
				}
				cancel = newcancel
				err := WConf([]byte("reloaded"), utils.NOW)
				if err != nil {
					return err
				}
			} else {
				logger.Warn("Error reloading configuration", "error", err)
			}

		case "confdir":
			if len(parts) == 2 {
				confdir = strings.TrimSpace(parts[1])
				if confined {
					confdir = filepath.Join("/tmp", "conf", confdir)
				}
			} else {
				return fmt.Errorf("empty confdir command")
			}
		case "consulparams":
			if len(parts) == 2 {
				newparams := consul.ConnParams{}
				err := json.Unmarshal([]byte(parts[1]), &newparams)
				if err == nil {
					//logger.Debug("Configuration child received consul params", "params", parts[1])
					params = newparams
				} else {
					return fmt.Errorf("error unmarshaling consulparams received from parent: %s", err.Error())
				}
			} else {
				return fmt.Errorf("empty consulparams command")
			}
		case "stop":
			if cancel != nil {
				cancel()
			}
			return nil
		default:
			return fmt.Errorf("unknown conf command")
		}

	}
	return scanner.Err()
}

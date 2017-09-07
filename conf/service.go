package conf

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/kardianos/osext"
	"github.com/stephane-martin/skewer/consul"
	"github.com/stephane-martin/skewer/utils"
)

type ConfigurationService struct {
	logger       log15.Logger
	loggerHandle int
	output       chan *BaseConfig
	stdin        io.WriteCloser
	confdir      string
	params       consul.ConnParams
	mu           *sync.Mutex
}

func NewConfigurationService(childLoggerHandle int, l log15.Logger) *ConfigurationService {
	s := &ConfigurationService{loggerHandle: childLoggerHandle}
	s.output = make(chan *BaseConfig)
	s.mu = &sync.Mutex{}
	s.logger = l
	return s
}

func (c *ConfigurationService) SetConfDir(cdir string) {
	c.confdir = cdir
}

func (c *ConfigurationService) SetConsulParams(params consul.ConnParams) {
	c.params = params
}

func (c *ConfigurationService) Stop() {
	c.mu.Lock()
	if c.stdin != nil {
		utils.W(c.stdin, "stop", utils.NOW)
		c.stdin = nil
	}
	c.mu.Unlock()
}

func (c *ConfigurationService) Reload() {
	c.mu.Lock()
	if c.stdin != nil {
		utils.W(c.stdin, "reload", utils.NOW)
	}
	c.mu.Unlock()
}

func (c *ConfigurationService) Chan() chan *BaseConfig {
	return c.output
}

func (c *ConfigurationService) Start() error {
	exe, err := osext.Executable()
	if err != nil {
		close(c.output)
		return err
	}

	cmd := &exec.Cmd{
		Args:       []string{"skewer-conf"},
		Path:       exe,
		Stderr:     os.Stderr,
		ExtraFiles: []*os.File{os.NewFile(uintptr(c.loggerHandle), "logger")},
	}

	c.stdin, err = cmd.StdinPipe()
	if err != nil {
		close(c.output)
		return err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		close(c.output)
		return err
	}

	err = cmd.Start()
	if err != nil {
		close(c.output)
		return err
	}

	startedChan := make(chan error)
	once := &sync.Once{}

	go func() {
		kill := false
		defer func() {
			if kill {
				c.mu.Lock()
				cmd.Process.Kill()
				c.mu.Unlock()
			}
			// TODO: after timeout kill anyway
			cmd.Wait()
			close(c.output)
		}()

		var command string
		scanner := bufio.NewScanner(stdout)
		scanner.Split(utils.PluginSplit)
		for scanner.Scan() {
			parts := strings.SplitN(scanner.Text(), " ", 2)
			command = parts[0]
			switch command {
			case "newconf":
				if len(parts) == 2 {
					newconf := BaseConfig{}
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
				msg := ""
				if len(parts) == 2 {
					msg = parts[1]
				} else {
					msg = "Received empty starterror from child"
					kill = true
				}
				c.logger.Error(msg)
				once.Do(func() { startedChan <- fmt.Errorf(msg); close(startedChan) })
				return
			case "reloaded":
				c.logger.Debug("Configuration child has been reloaded")
			default:
				msg := "Unknown command received from configuration child"
				c.logger.Error(msg, "command", command)
				kill = true
				once.Do(func() { startedChan <- fmt.Errorf(msg + ": " + command); close(startedChan) })
				return
			}

		}
		// TODO: check scanner error

	}()

	cparams, _ := json.Marshal(c.params)

	c.mu.Lock()
	utils.W(c.stdin, "confdir", []byte(c.confdir))
	utils.W(c.stdin, "consulparams", cparams)
	utils.W(c.stdin, "start", utils.NOW)
	c.mu.Unlock()
	err = <-startedChan
	if err != nil {
		c.Stop()
	}
	return err

}

type ConfigurationProvider struct {
	params  consul.ConnParams
	confdir string
	logger  log15.Logger
}

func (c *ConfigurationProvider) Write(newconf *BaseConfig) {
	b, err := json.Marshal(newconf)
	if err == nil {
		utils.W(os.Stdout, "newconf", b)
	} else {
		c.logger.Warn("Error marshaling new configuration", "error", err)
	}
}

func (c *ConfigurationProvider) Launch(logger log15.Logger) error {
	c.logger = logger

	var ctx context.Context
	var cancel context.CancelFunc
	var updated chan *BaseConfig
	var gconf *BaseConfig

	start := func() error {
		var err error
		ctx, cancel = context.WithCancel(context.Background())
		gconf, updated, err = InitLoad(ctx, c.confdir, c.params, logger)
		if err == nil {
			confb, err := json.Marshal(gconf)
			if err == nil {
				utils.W(os.Stdout, "started", utils.NOW)
				utils.W(os.Stdout, "newconf", confb)
				go func() {
					for {
						select {
						case <-ctx.Done():
							return
						case newconf, more := <-updated:
							if more {
								confb, err := json.Marshal(newconf)
								if err == nil {
									utils.W(os.Stdout, "newconf", confb)
								} else {
									logger.Warn("Error serializing new configuration", "error", err)
								}
							} else {
								return
							}
						}
					}
				}()
			} else {
				return err
			}
		} else {
			return err
		}
		return nil
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(utils.PluginSplit)
	var command string

	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), " ", 2)
		command = parts[0]

		switch command {
		case "start":
			err := start()
			if err != nil {
				utils.W(os.Stdout, "starterror", []byte(err.Error()))
				return err
			}

		case "reload":
			oldcancel := cancel
			err := start()
			if err == nil {
				if oldcancel != nil {
					oldcancel()
				}
				utils.W(os.Stdout, "reloaded", utils.NOW)
			} else {
				logger.Warn("Error reloading configuration", "error", err)
			}

		case "confdir":
			if len(parts) == 2 {
				c.confdir = strings.TrimSpace(parts[1])
			} else {
				return fmt.Errorf("Empty confdir command")
			}
		case "consulparams":
			if len(parts) == 2 {
				params := consul.ConnParams{}
				err := json.Unmarshal([]byte(parts[1]), &params)
				if err == nil {
					c.params = params
				} else {
					return fmt.Errorf("Error unmarshaling consulparams received from parent: %s", err.Error())
				}
			} else {
				return fmt.Errorf("Empty consulparams command")
			}
		case "stop":
			return nil
		default:
			return fmt.Errorf("Unknown command")
		}

	}
	return scanner.Err()
}

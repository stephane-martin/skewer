package conf

import (
	"bytes"
	"fmt"
	"net"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/hashicorp/errwrap"
	"github.com/spf13/viper"
)

type GlobalConfig struct {
	Syslog SyslogConfig `mapstructure:"syslog" toml:"syslog"`
	Kafka  KafkaConfig  `mapstructure:"kafka" toml:"kafka"`
}

type KafkaConfig struct {
	Brokers  []string `mapstructure:"brokers" toml:"brokers"`
	ClientID string   `mapstructure:"client_id" toml:"client_id"`
}

type SyslogConfig struct {
	Port       int    `mapstructure:"port" toml:"port"`
	BindAddr   string `mapstructure:"bind_addr" toml:"bind_addr"`
	BindIP     net.IP `toml:"-"`
	ListenAddr string `toml:"-"`
}

func New() *GlobalConfig {
	brokers := []string{}
	kafka := KafkaConfig{Brokers: brokers, ClientID: ""}
	syslog := SyslogConfig{}
	conf := GlobalConfig{Syslog: syslog, Kafka: kafka}
	return &conf
}

func Default() *GlobalConfig {
	v := viper.New()
	SetDefaults(v)
	c := New()
	v.Unmarshal(c)
	c.Complete()
	return c
}

func (c *GlobalConfig) String() string {
	return c.Export()
}

func Load(dirname string) (*GlobalConfig, error) {
	v := viper.New()
	SetDefaults(v)
	v.SetConfigName("relp2kafka")

	dirname = strings.TrimSpace(dirname)
	if len(dirname) > 0 {
		v.AddConfigPath(dirname)
	}
	if dirname != "/nonexistent" {
		v.AddConfigPath("/etc")
	}

	err := v.ReadInConfig()
	if err != nil {
		switch err.(type) {
		default:
			return nil, errwrap.Wrapf("Error reading the configuration file", err)
		case viper.ConfigFileNotFoundError:
			// log.Log.WithError(err).Debug("No configuration file was found")
		}
	}

	c := New()
	err = v.Unmarshal(c)
	if err != nil {
		return nil, errwrap.Wrapf("Syntax error in configuration file: {{err}}", err)
	}
	err = c.Complete()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *GlobalConfig) Export() string {
	buf := new(bytes.Buffer)
	encoder := toml.NewEncoder(buf)
	encoder.Encode(*c)
	return buf.String()
}

func (c *GlobalConfig) Complete() error {
	c.Syslog.BindIP = net.ParseIP(c.Syslog.BindAddr)
	if c.Syslog.BindIP == nil {
		return fmt.Errorf("syslog.bind_addr is not an IP address")
	}
	if c.Syslog.BindIP.IsUnspecified() {
		c.Syslog.ListenAddr = fmt.Sprintf(":%d", c.Syslog.Port)
	} else {
		c.Syslog.ListenAddr = fmt.Sprintf("%s:%d", c.Syslog.BindIP.String(), c.Syslog.Port)
	}
	return nil
}

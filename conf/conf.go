package conf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"text/template"
	"time"

	sarama "gopkg.in/Shopify/sarama.v1"

	"github.com/BurntSushi/toml"
	"github.com/hashicorp/consul/api"
	"github.com/inconshreveable/log15"
	"github.com/spf13/viper"
	"github.com/stephane-martin/relp2kafka/consul"
)

type BaseConfig struct {
	Syslog   []SyslogConfig  `mapstructure:"syslog" toml:"syslog"`
	Kafka    KafkaConfig     `mapstructure:"kafka" toml:"kafka"`
	Store    StoreConfig     `mapstructure:"store" toml:"store"`
	Parsers  []ParserConfig  `mapstructure:"parser" toml:"parser"`
	Watchers []WatcherConfig `mapstructure:"watcher" toml:"watcher"`
	Journald JournaldConfig  `mapstructure:"journald" toml:"journald"`
}

type GConfig struct {
	BaseConfig
	Dirname      string
	Updated      chan bool
	ConsulClient *api.Client
	ConsulPrefix string
	ConsulParams consul.ConnParams
	Logger       log15.Logger
}

func newBaseConf() *BaseConfig {
	brokers := []string{}
	kafka := KafkaConfig{Brokers: brokers}
	syslog := []SyslogConfig{}
	parsers := []ParserConfig{}
	baseConf := BaseConfig{Syslog: syslog, Kafka: kafka, Parsers: parsers}
	return &baseConf
}

func newConf() *GConfig {
	baseConf := newBaseConf()
	conf := GConfig{BaseConfig: *baseConf}
	conf.Updated = make(chan bool, 10)
	conf.Logger = log15.New()
	return &conf
}

func Default() *GConfig {
	v := viper.New()
	SetDefaults(v)
	c := newConf()
	v.Unmarshal(c)
	c.Complete()
	return c
}

func (c *GConfig) String() string {
	return c.Export()
}

type WatcherConfig struct {
	Filename string `mapstructure:"filename" toml:"filename"`
	Whence   int    `mapstructure:"whence" toml:"whence"`
}

type ParserConfig struct {
	Name string `mapstructure:"name" toml:"name"`
	Func string `mapstructure:"func" toml:"func"`
}

type StoreConfig struct {
	Dirname string `mapstructure:"dirname" toml:"dirname"`
	Maxsize int64  `mapstructure:"max_size" toml:"max_size"`
	FSync   bool   `mapstructure:"fsync" toml:"fsync"`
}

type KafkaVersion [4]int

var V0_8_2_0 = KafkaVersion{0, 8, 2, 0}
var V0_8_2_1 = KafkaVersion{0, 8, 2, 1}
var V0_8_2_2 = KafkaVersion{0, 8, 2, 2}
var V0_9_0_0 = KafkaVersion{0, 9, 0, 0}
var V0_9_0_1 = KafkaVersion{0, 9, 0, 1}
var V0_10_0_0 = KafkaVersion{0, 10, 0, 0}
var V0_10_0_1 = KafkaVersion{0, 10, 0, 1}
var V0_10_1_0 = KafkaVersion{0, 10, 1, 0}
var V0_10_2_0 = KafkaVersion{0, 10, 2, 0}

func ParseVersion(v string) (skv sarama.KafkaVersion, e error) {
	var ver KafkaVersion
	for i, n := range strings.SplitN(v, ".", 4) {
		ver[i], e = strconv.Atoi(n)
		if e != nil {
			return skv, ConfigurationCheckError{ErrString: fmt.Sprintf("Kafka Version has invalid format: '%s'", v)}
		}
	}
	return ver.ToSaramaVersion()
}

func (l KafkaVersion) ToSaramaVersion() (v sarama.KafkaVersion, e error) {
	if l.Greater(V0_10_2_0) {
		return sarama.V0_10_2_0, nil
	}
	if l.Greater(V0_10_1_0) {
		return sarama.V0_10_1_0, nil
	}
	if l.Greater(V0_10_0_1) {
		return sarama.V0_10_1_0, nil
	}
	if l.Greater(V0_10_0_0) {
		return sarama.V0_10_0_0, nil
	}
	if l.Greater(V0_9_0_1) {
		return sarama.V0_9_0_1, nil
	}
	if l.Greater(V0_9_0_0) {
		return sarama.V0_9_0_0, nil
	}
	if l.Greater(V0_8_2_2) {
		return sarama.V0_8_2_2, nil
	}
	if l.Greater(V0_8_2_1) {
		return sarama.V0_8_2_1, nil
	}
	if l.Greater(V0_8_2_0) {
		return sarama.V0_8_2_0, nil
	}
	return v, ConfigurationCheckError{ErrString: "Minimal Kafka version is 0.8.2.0"}
}

func (l KafkaVersion) Greater(r KafkaVersion) bool {
	if l[0] > r[0] {
		return true
	}
	if l[0] < r[0] {
		return false
	}
	if l[1] > r[1] {
		return true
	}
	if l[1] < r[1] {
		return false
	}
	if l[2] > r[2] {
		return true
	}
	if l[2] < r[2] {
		return false
	}
	if l[3] >= r[3] {
		return true
	}
	return false
}

type KafkaConfig struct {
	Brokers                  []string                `mapstructure:"brokers" toml:"brokers"`
	ClientID                 string                  `mapstructure:"client_id" toml:"client_id"`
	Version                  string                  `mapstructure:"version" toml:"version"`
	ChannelBufferSize        int                     `mapstructure:"channel_buffer_size" toml:"channel_buffer_size"`
	MaxOpenRequests          int                     `mapstructure:"max_open_requests" toml:"max_open_requests"`
	DialTimeout              time.Duration           `mapstructure:"dial_timeout" toml:"dial_timeout"`
	ReadTimeout              time.Duration           `mapstructure:"read_timeout" toml:"read_timeout"`
	WriteTimeout             time.Duration           `mapstructure:"write_timeout" toml:"write_timeout"`
	KeepAlive                time.Duration           `mapstructure:"keepalive" toml:"keepalive"`
	MetadataRetryMax         int                     `mapstructure:"metadata_retry_max" toml:"metadata_retry_max"`
	MetadataRetryBackoff     time.Duration           `mapstructure:"metadata_retry_backoff" toml:"metadata_retry_backoff"`
	MetadataRefreshFrequency time.Duration           `mapstructure:"metadata_refresh_frequency" toml:"metadata_refresh_frequency"`
	MessageBytesMax          int                     `mapstructure:"message_bytes_max" toml:"message_bytes_max"`
	RequiredAcks             int16                   `mapstructure:"required_acks" toml:"required_acks"`
	ProducerTimeout          time.Duration           `mapstructure:"producer_timeout" toml:"producer_timeout"`
	Compression              string                  `mapstructure:"compression" toml:"compression"`
	FlushBytes               int                     `mapstructure:"flush_bytes" toml:"flush_bytes"`
	FlushMessages            int                     `mapstructure:"flush_messages" toml:"flush_messages"`
	FlushFrequency           time.Duration           `mapstructure:"flush_frequency" toml:"flush_frequency"`
	FlushMessagesMax         int                     `mapstructure:"flush_messages_max" toml:"flush_messages_max"`
	RetrySendMax             int                     `mapstructure:"retry_send_max" toml:"retry_send_max"`
	RetrySendBackoff         time.Duration           `mapstructure:"retry_send_backoff" toml:"retry_send_backoff"`
	pVersion                 sarama.KafkaVersion     `toml:"-"`
	pCompression             sarama.CompressionCodec `toml:"-"`
}

type JournaldConfig struct {
	Enabled       bool
	TopicTmpl     string `mapstructure:"topic_tmpl" toml:"topic_tmpl"`
	TopicFunc     string `mapstructure:"topic_function" toml:"topic_function"`
	PartitionTmpl string `mapstructure:"partition_key_tmpl" toml:"partition_key_tmpl"`
	PartitionFunc string `mapstructure:"partition_key_func" toml:"partition_key_func"`
	FilterFunc    string `mapstructure:"filter_func" toml:"filter_func"`
}

type SyslogConfig struct {
	Port            int           `mapstructure:"port" toml:"port" json:"port"`
	BindAddr        string        `mapstructure:"bind_addr" toml:"bind_addr" json:"bind_addr"`
	UnixSocketPath  string        `mapstructure:"unix_socket_path" toml:"unix_socket_path" json:"unix_socket_path"`
	Format          string        `mapstructure:"format" toml:"format" json:"format"`
	TopicTmpl       string        `mapstructure:"topic_tmpl" toml:"topic_tmpl" json:"topic_tmpl"`
	TopicFunc       string        `mapstructure:"topic_function" toml:"topic_function" json:"topic_function"`
	PartitionTmpl   string        `mapstructure:"partition_key_tmpl" toml:"partition_key_tmpl" json:"partition_key_tmpl"`
	PartitionFunc   string        `mapstructure:"partition_key_func" toml:"partition_key_func" json:"partition_key_func"`
	FilterFunc      string        `mapstructure:"filter_func" toml:"filter_func" json:"filter_func"`
	Protocol        string        `mapstructure:"protocol" toml:"protocol" json:"protocol"`
	DontParseSD     bool          `mapstructure:"dont_parse_structured_data" toml:"dont_parse_structured_data" json:"dont_parse_structured_data"`
	KeepAlive       bool          `mapstructure:"keepalive" toml:"keepalive" json:"keepalive"`
	KeepAlivePeriod time.Duration `mapstructure:"keepalive_period" toml:"keepalive_period" json:"keepalive_period"`
	Timeout         time.Duration `mapstructure:"timeout" toml:"timeout" json:"timeout"`

	// Partitioner ?
}

func (c *SyslogConfig) GetListenAddr() (string, error) {
	if len(c.UnixSocketPath) > 0 {
		return "", nil
	}
	bindIP := net.ParseIP(c.BindAddr)
	if bindIP == nil {
		return "", fmt.Errorf("bind_addr is not an IP address: %s", c.BindAddr)
	}

	if bindIP.IsUnspecified() {
		return fmt.Sprintf(":%d", c.Port), nil
	} else {
		return fmt.Sprintf("%s:%d", bindIP.String(), c.Port), nil
	}
}

func (c *SyslogConfig) Export() []byte {
	b, _ := json.Marshal(c)
	return b
}

func ImportSyslogConfig(data []byte) (*SyslogConfig, error) {
	c := SyslogConfig{}
	err := json.Unmarshal(data, &c)
	if err != nil {
		return nil, fmt.Errorf("Can't unmarshal the syslog config: %s", err.Error())
	}
	return &c, nil
}

func (c *KafkaConfig) GetSaramaConfig() *sarama.Config {
	s := sarama.NewConfig()
	s.Net.MaxOpenRequests = c.MaxOpenRequests
	s.Net.DialTimeout = c.DialTimeout
	s.Net.ReadTimeout = c.ReadTimeout
	s.Net.WriteTimeout = c.WriteTimeout
	s.Net.KeepAlive = c.KeepAlive
	s.Metadata.Retry.Backoff = c.MetadataRetryBackoff
	s.Metadata.Retry.Max = c.MetadataRetryMax
	s.Metadata.RefreshFrequency = c.MetadataRefreshFrequency
	s.Producer.MaxMessageBytes = c.MessageBytesMax
	s.Producer.RequiredAcks = sarama.RequiredAcks(c.RequiredAcks)
	s.Producer.Timeout = c.ProducerTimeout
	s.Producer.Compression = c.pCompression
	s.Producer.Return.Errors = true
	s.Producer.Return.Successes = true
	s.Producer.Flush.Bytes = c.FlushBytes
	s.Producer.Flush.Frequency = c.FlushFrequency
	s.Producer.Flush.Messages = c.FlushMessages
	s.Producer.Flush.MaxMessages = c.FlushMessagesMax
	s.Producer.Retry.Backoff = c.RetrySendBackoff
	s.Producer.Retry.Max = c.RetrySendMax
	s.ClientID = c.ClientID
	s.ChannelBufferSize = c.ChannelBufferSize
	s.Version = c.pVersion
	// MetricRegistry ?
	// partitioner ?
	return s
}

func (c *KafkaConfig) GetAsyncProducer() (sarama.AsyncProducer, error) {
	p, err := sarama.NewAsyncProducer(c.Brokers, c.GetSaramaConfig())
	if err == nil {
		return p, nil
	}
	return nil, KafkaError{Err: err}
}

func (c *KafkaConfig) GetClient() (sarama.Client, error) {
	cl, err := sarama.NewClient(c.Brokers, c.GetSaramaConfig())
	if err == nil {
		return cl, nil
	}
	return nil, KafkaError{Err: err}
}

func InitLoad(dirname string, params consul.ConnParams, prefix string, logger log15.Logger) (c *GConfig, stopWatchChan chan bool, err error) {
	var firstResults map[string]string
	var consulResults chan map[string]string

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

	err = v.ReadInConfig()
	if err != nil {
		switch err.(type) {
		default:
			return nil, nil, ConfigurationReadError{err}
		case viper.ConfigFileNotFoundError:
			logger.Info("No configuration file was found")
		}
	}

	baseConf := newBaseConf()
	err = v.Unmarshal(baseConf)
	if err != nil {
		return nil, nil, ConfigurationSyntaxError{Err: err, Filename: v.ConfigFileUsed()}
	}

	c = &GConfig{BaseConfig: *baseConf}
	c.Updated = make(chan bool, 10)
	c.Dirname = dirname
	c.ConsulParams = params
	c.ConsulPrefix = prefix
	c.Logger = logger

	consulAddr := strings.TrimSpace(params.Address)
	if len(consulAddr) > 0 {
		var clt *api.Client
		clt, err = consul.NewClient(params)
		if err == nil {
			c.ConsulClient = clt
			consulResults = make(chan map[string]string, 10)
			firstResults, stopWatchChan, err = consul.WatchTree(c.ConsulClient, c.ConsulPrefix, consulResults, logger)
			if err == nil {
				err = c.ParseParamsFromConsul(firstResults)
				if err != nil {
					c.Logger.Error("Error decoding configuration from Consul", "error", err)
				}
			} else {
				c.Logger.Error("Error reading from Consul", "error", err)
				consulResults = nil
			}
		} else {
			logger.Error("Error creating Consul client: configuration will not be fetched from Consul", "error", err)
		}
	} else {
		logger.Info("Configuration is not fetched from Consul")
	}

	err = c.Complete()
	if err != nil {
		if stopWatchChan != nil {
			close(stopWatchChan)
		}
		return nil, nil, err
	}

	if consulResults != nil {
		// watch for updates from Consul
		// (c.Updated is not modified or closed, same channel for the new config)
		go func() {
			for result := range consulResults {
				oldDirname := c.Store.Dirname
				var newConfig *GConfig
				*newConfig = *c
				err := newConfig.ParseParamsFromConsul(result)
				if err == nil {
					err = newConfig.Complete()
					newConfig.Store.Dirname = oldDirname
					if err == nil {
						*c = *newConfig
						c.Updated <- true
					} else {
						logger.Error("Error updating conf from Consul", "error", err)
					}
				} else {
					c.Logger.Error("Error decoding conf from Consul", "error", err)
				}
			}
			close(c.Updated)
		}()
	}

	return c, stopWatchChan, nil
}

func (c *GConfig) ParseParamsFromConsul(params map[string]string) error {
	syslogConfMap := map[string]map[string]string{}
	journaldConf := map[string]string{}
	kafkaConf := map[string]string{}
	storeConf := map[string]string{}
	parsersConfMap := map[string]map[string]string{}
	prefixLen := len(c.ConsulPrefix)

	for k, v := range params {
		k = strings.Trim(k[prefixLen:], "/")
		splits := strings.Split(k, "/")
		switch splits[0] {
		case "syslog":
			if len(splits) == 3 {
				if _, ok := syslogConfMap[splits[1]]; !ok {
					syslogConfMap[splits[1]] = map[string]string{}
				}
				syslogConfMap[splits[1]][splits[2]] = v
			} else {
				c.Logger.Debug("Ignoring Consul KV", "key", k, "value", v)
			}
		case "journald":
			if len(splits) == 2 {
				journaldConf[splits[1]] = v
			} else {
				c.Logger.Debug("Ignoring Consul KV", "key", k, "value", v)
			}
		case "kafka":
			if len(splits) == 2 {
				kafkaConf[splits[1]] = v
			} else {
				c.Logger.Debug("Ignoring Consul KV", "key", k, "value", v)
			}
		case "store":
			if len(splits) == 2 {
				storeConf[splits[1]] = v
			} else {
				c.Logger.Debug("Ignoring Consul KV", "key", k, "value", v)
			}
		case "parsers":
			if len(splits) == 3 {
				if _, ok := parsersConfMap[splits[1]]; !ok {
					parsersConfMap[splits[1]] = map[string]string{}
				}
				parsersConfMap[splits[1]][splits[2]] = v
			} else {
				c.Logger.Debug("Ignoring Consul KV", "key", k, "value", v)
			}
		default:
			c.Logger.Debug("Ignoring Consul KV", "key", k, "value", v)
		}
	}

	var vi *viper.Viper

	syslogConfs := []SyslogConfig{}
	for _, syslogConf := range syslogConfMap {
		vi = viper.New()
		for k, v := range syslogConf {
			vi.Set(k, v)
		}
		sconf := SyslogConfig{}
		err := vi.Unmarshal(&sconf)
		if err == nil {
			syslogConfs = append(syslogConfs, sconf)
		} else {
			return err
		}
	}

	parsersConf := []ParserConfig{}
	for parserName, pConf := range parsersConfMap {
		parserConf := ParserConfig{Name: parserName}
		vi := viper.New()
		for k, v := range pConf {
			vi.Set(k, v)
		}
		err := vi.Unmarshal(&parserConf)
		if err == nil {
			parsersConf = append(parsersConf, parserConf)
		} else {
			return err
		}
	}

	jconf := JournaldConfig{}
	if len(journaldConf) > 0 {
		vi = viper.New()
		SetJournaldDefaults(vi, false)
		for k, v := range journaldConf {
			vi.Set(k, v)
		}
		err := vi.Unmarshal(&jconf)
		if err != nil {
			return err
		}
	}

	kconf := KafkaConfig{}
	if len(kafkaConf) > 0 {
		vi = viper.New()
		SetKafkaDefaults(vi, false)
		for k, v := range kafkaConf {
			vi.Set(k, v)
		}
		err := vi.Unmarshal(&kconf)
		if err != nil {
			return err
		}
	}

	sconf := StoreConfig{}
	if len(storeConf) > 0 {
		vi = viper.New()
		SetStoreDefaults(vi, false)
		for k, v := range storeConf {
			vi.Set(k, v)
		}
		err := vi.Unmarshal(&sconf)
		if err != nil {
			return err
		}
	}

	c.Syslog = append(c.Syslog, syslogConfs...)
	c.Parsers = append(c.Parsers, parsersConf...)
	if len(kafkaConf) > 0 {
		c.Kafka = kconf
	}
	if len(storeConf) > 0 {
		c.Store = sconf
	}
	if len(journaldConf) > 0 {
		c.Journald = jconf
	}

	return nil
}

func (c *GConfig) Reload() (newConf *GConfig, stopWatchChan chan bool, err error) {
	newConf, stopWatchChan, err = InitLoad(c.Dirname, c.ConsulParams, c.ConsulPrefix, c.Logger)
	if err != nil {
		return nil, nil, err
	}
	newConf.Store = c.Store // we don't change the location of the badger databases when doing a reload
	return newConf, stopWatchChan, nil
}

func (c *GConfig) Export() string {
	buf := new(bytes.Buffer)
	encoder := toml.NewEncoder(buf)
	encoder.Encode(c.BaseConfig)
	return buf.String()
}

func (c *GConfig) Complete() (err error) {
	parsersNames := map[string]bool{}
	for _, parserConf := range c.Parsers {
		name := strings.TrimSpace(parserConf.Name)
		switch name {
		case "rfc5424", "rfc3164", "json", "auto":
			return ConfigurationCheckError{ErrString: "Parser configuration must not use a reserved name"}
		case "":
			return ConfigurationCheckError{ErrString: "Empty parser name"}
		default:
			if _, ok := parsersNames[name]; ok {
				return ConfigurationCheckError{ErrString: "The same parser name is used multiple times"}
			}
			f := strings.TrimSpace(parserConf.Func)
			if len(f) == 0 {
				return ConfigurationCheckError{ErrString: "Empty parser func"}
			}
			parsersNames[name] = true
		}
	}

	switch c.Kafka.Compression {
	case "snappy":
		c.Kafka.pCompression = sarama.CompressionSnappy
	case "gzip":
		c.Kafka.pCompression = sarama.CompressionGZIP
	case "lz4":
		c.Kafka.pCompression = sarama.CompressionLZ4
	default:
		c.Kafka.pCompression = sarama.CompressionNone
	}

	c.Kafka.pVersion, err = ParseVersion(c.Kafka.Version)
	if err != nil {
		return ConfigurationCheckError{ErrString: "Kafka version can't be parsed", Err: err}
	}

	if len(c.Syslog) == 0 {
		syslogConf := SyslogConfig{
			Port:          2514,
			BindAddr:      "127.0.0.1",
			Format:        "rfc5424",
			Protocol:      "relp",
			TopicTmpl:     "rsyslog-{{.Appname}}",
			PartitionTmpl: "mypk-{{.Hostname}}",
		}
		c.Syslog = []SyslogConfig{syslogConf}
	}

	for i, syslogConf := range c.Syslog {
		switch syslogConf.Protocol {
		case "relp", "tcp", "udp":
		default:
			return ConfigurationCheckError{ErrString: "Unknown protocol"}
		}
		if syslogConf.UnixSocketPath == "" {
			if syslogConf.BindAddr == "" {
				c.Syslog[i].BindAddr = "127.0.0.1"
			}
			if syslogConf.Port == 0 {
				switch c.Syslog[i].Protocol {
				case "relp":
					c.Syslog[i].Port = 2514
				case "tcp", "udp":
					c.Syslog[i].Port = 1514
				default:
					return ConfigurationCheckError{ErrString: "Unknown protocol"}
				}
			}
		}

		if syslogConf.Format == "" {
			c.Syslog[i].Format = "auto"
		}
		if syslogConf.TopicTmpl == "" {
			c.Syslog[i].TopicTmpl = "rsyslog-{{.Appname}}"
		}
		if syslogConf.PartitionTmpl == "" {
			c.Syslog[i].PartitionTmpl = "mypk-{{.Hostname}}"
		}
		if syslogConf.KeepAlivePeriod == 0 {
			c.Syslog[i].KeepAlivePeriod = 30 * time.Second
		}
		if syslogConf.Timeout == 0 {
			c.Syslog[i].Timeout = time.Minute
		}

		if len(c.Syslog[i].TopicTmpl) > 0 {
			_, err = template.New("topic").Parse(c.Syslog[i].TopicTmpl)
			if err != nil {
				return ConfigurationCheckError{ErrString: "Error compiling the topic template", Err: err}
			}
		}
		if len(c.Syslog[i].PartitionTmpl) > 0 {
			_, err = template.New("partition").Parse(c.Syslog[i].PartitionTmpl)
			if err != nil {
				return ConfigurationCheckError{ErrString: "Error compiling the partition key template", Err: err}
			}
		}

		_, err = c.Syslog[i].GetListenAddr()
		if err != nil {
			return ConfigurationCheckError{Err: err}
		}
	}

	for _, syslogConf := range c.Syslog {
		switch syslogConf.Format {
		case "rfc5424", "rfc3164", "json", "auto":
		default:
			if _, ok := parsersNames[syslogConf.Format]; !ok {
				return ConfigurationCheckError{ErrString: "Unknown syslog format"}
			}
		}
	}

	if c.Journald.Enabled {
		var err error

		if len(c.Journald.TopicTmpl) > 0 {
			_, err = template.New("journaldtopic").Parse(c.Journald.TopicTmpl)
			if err != nil {
				return ConfigurationCheckError{ErrString: "Error compiling the topic template", Err: err}
			}
		}
		if len(c.Journald.PartitionTmpl) > 0 {
			_, err = template.New("journaldpartition").Parse(c.Journald.PartitionTmpl)
			if err != nil {
				return ConfigurationCheckError{ErrString: "Error compiling the partition key template", Err: err}
			}
		}
	}

	return nil
}

package conf

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
	"text/template"
	"time"

	sarama "gopkg.in/Shopify/sarama.v1"

	"github.com/BurntSushi/toml"
	"github.com/spf13/viper"
)

type GlobalConfig struct {
	Syslog []SyslogConfig `mapstructure:"syslog" toml:"syslog"`
	Kafka  KafkaConfig    `mapstructure:"kafka" toml:"kafka"`
	Store  StoreConfig    `mapstructure:"store" toml:"store"`
}

type StoreConfig struct {
	Dirname string `mapstructure:"dirname" toml:"dirname"`
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
			return skv, ConfigurationCheckError{ErrString: fmt.Sprintf("Kafka Version has invalid format: %s", v)}
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

type SyslogConfig struct {
	Port                 int                `mapstructure:"port" toml:"port"`
	BindAddr             string             `mapstructure:"bind_addr" toml:"bind_addr"`
	Format               string             `mapstructure:"format" toml:"format"`
	TopicTmpl            string             `mapstructure:"topic_tmpl" toml:"topic_tmpl"`
	PartitionTmpl        string             `mapstructure:"partition_key_tmpl" toml:"partition_key_tmpl"`
	Protocol             string             `mapstructure:"protocol" toml:"protocol"`
	TopicTemplate        *template.Template `toml:"-"`
	PartitionKeyTemplate *template.Template `toml:"-"`
	BindIP               net.IP             `toml:"-"`
	ListenAddr           string             `toml:"-"`
	// Filter ?
	// Partitioner ?
	// Topic function ?
	// Partition key function ?
}

func (c *GlobalConfig) GetSaramaConfig() *sarama.Config {
	s := sarama.NewConfig()
	s.Net.MaxOpenRequests = c.Kafka.MaxOpenRequests
	s.Net.DialTimeout = c.Kafka.DialTimeout
	s.Net.ReadTimeout = c.Kafka.ReadTimeout
	s.Net.WriteTimeout = c.Kafka.WriteTimeout
	s.Net.KeepAlive = c.Kafka.KeepAlive
	s.Metadata.Retry.Backoff = c.Kafka.MetadataRetryBackoff
	s.Metadata.Retry.Max = c.Kafka.MetadataRetryMax
	s.Metadata.RefreshFrequency = c.Kafka.MetadataRefreshFrequency
	s.Producer.MaxMessageBytes = c.Kafka.MessageBytesMax
	s.Producer.RequiredAcks = sarama.RequiredAcks(c.Kafka.RequiredAcks)
	s.Producer.Timeout = c.Kafka.ProducerTimeout
	s.Producer.Compression = c.Kafka.pCompression
	s.Producer.Return.Errors = true
	s.Producer.Return.Successes = true
	s.Producer.Flush.Bytes = c.Kafka.FlushBytes
	s.Producer.Flush.Frequency = c.Kafka.FlushFrequency
	s.Producer.Flush.Messages = c.Kafka.FlushMessages
	s.Producer.Flush.MaxMessages = c.Kafka.FlushMessagesMax
	s.Producer.Retry.Backoff = c.Kafka.RetrySendBackoff
	s.Producer.Retry.Max = c.Kafka.RetrySendMax
	s.ClientID = c.Kafka.ClientID
	s.ChannelBufferSize = c.Kafka.ChannelBufferSize
	s.Version = c.Kafka.pVersion
	// MetricRegistry ?
	// partitioner ?
	return s
}

func (c *GlobalConfig) GetKafkaAsyncProducer() (sarama.AsyncProducer, error) {
	p, err := sarama.NewAsyncProducer(c.Kafka.Brokers, c.GetSaramaConfig())
	if err == nil {
		return p, nil
	}
	return nil, KafkaError{Err: err}
}

func (c *GlobalConfig) GetKafkaClient() (sarama.Client, error) {
	cl, err := sarama.NewClient(c.Kafka.Brokers, c.GetSaramaConfig())
	if err == nil {
		return cl, nil
	}
	return nil, KafkaError{Err: err}
}

func New() *GlobalConfig {
	brokers := []string{}
	kafka := KafkaConfig{Brokers: brokers, ClientID: ""}
	syslog := []SyslogConfig{}
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
			return nil, ConfigurationReadError{err}
		case viper.ConfigFileNotFoundError:
			// log.Log.WithError(err).Debug("No configuration file was found")
		}
	}
	c := New()
	err = v.Unmarshal(c)
	if err != nil {
		return nil, ConfigurationSyntaxError{Err: err, Filename: v.ConfigFileUsed()}
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

func (c *GlobalConfig) Complete() (err error) {
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
		return err
	}

	if len(c.Syslog) == 0 {
		syslogConf := SyslogConfig{
			Port:          2514,
			BindAddr:      "127.0.0.1",
			Format:        "rfc5424",
			Protocol:      "relp",
			TopicTmpl:     "rsyslog-{{.Fields.Appname}}",
			PartitionTmpl: "mypk-{{.Fields.Hostname}}",
		}
		c.Syslog = []SyslogConfig{syslogConf}
	}
	for i, syslogConf := range c.Syslog {
		if syslogConf.Port == 0 {
			c.Syslog[i].Port = 2514
		}
		if syslogConf.BindAddr == "" {
			c.Syslog[i].BindAddr = "127.0.0.1"
		}
		if syslogConf.Format == "" {
			c.Syslog[i].Format = "rfc5424"
		}
		if syslogConf.Protocol == "" {
			c.Syslog[i].Protocol = "relp"
		}
		if syslogConf.TopicTmpl == "" {
			c.Syslog[i].TopicTmpl = "rsyslog-{{.Fields.Appname}}"
		}
		if syslogConf.PartitionTmpl == "" {
			c.Syslog[i].PartitionTmpl = "mypk-{{.Fields.Hostname}}"
		}

		c.Syslog[i].TopicTemplate, err = template.New("topic").Parse(c.Syslog[i].TopicTmpl)
		if err != nil {
			return ConfigurationCheckError{ErrString: "Error compiling the topic template", Err: err}
		}
		c.Syslog[i].PartitionKeyTemplate, err = template.New("partition").Parse(c.Syslog[i].PartitionTmpl)
		if err != nil {
			return ConfigurationCheckError{ErrString: "Error compiling the partition key template", Err: err}
		}

		c.Syslog[i].BindIP = net.ParseIP(c.Syslog[i].BindAddr)
		if c.Syslog[i].BindIP == nil {
			return ConfigurationCheckError{ErrString: fmt.Sprintf("bind_addr is not an IP address: %s", c.Syslog[i].BindAddr)}
			return fmt.Errorf("syslog.bind_addr is not an IP address")
		}

		if c.Syslog[i].BindIP.IsUnspecified() {
			c.Syslog[i].ListenAddr = fmt.Sprintf(":%d", c.Syslog[i].Port)
		} else {
			c.Syslog[i].ListenAddr = fmt.Sprintf("%s:%d", c.Syslog[i].BindIP.String(), c.Syslog[i].Port)
		}

	}

	return nil
}

package conf

import (
	"time"
)

type BaseConfig struct {
	Syslog   []SyslogConfig `mapstructure:"syslog" toml:"syslog" json:"syslog"`
	Kafka    KafkaConfig    `mapstructure:"kafka" toml:"kafka" json:"kafka"`
	Store    StoreConfig    `mapstructure:"store" toml:"store" json:"store"`
	Parsers  []ParserConfig `mapstructure:"parser" toml:"parser" json:"parser"`
	Journald JournaldConfig `mapstructure:"journald" toml:"journald" json:"journald"`
	Metrics  MetricsConfig  `mapstructure:"metrics" toml:"metrics" json:"metrics"`
}

type MetricsConfig struct {
	Path string `mapstructure:"path" toml:"path" json:"path"`
	Port int    `mapstructure:"port" toml:"port" json:"port"`
}

type WatcherConfig struct {
	Filename string `mapstructure:"filename" toml:"filename" json:"filename"`
	Whence   int    `mapstructure:"whence" toml:"whence" json:"whence"`
}

type ParserConfig struct {
	Name string `mapstructure:"name" toml:"name" json:"name"`
	Func string `mapstructure:"func" toml:"func" json:"func"`
}

type StoreConfig struct {
	Dirname string   `mapstructure:"-" toml:"-" json:"dirname"`
	Maxsize int64    `mapstructure:"max_size" toml:"max_size" json:"max_size"`
	FSync   bool     `mapstructure:"fsync" toml:"fsync" json:"fsync"`
	Secret  string   `mapstructure:"secret" toml:"-" json:"secret"`
	SecretB [32]byte `mapstructure:"-" toml:"-" json:"secretb"`
}

type KafkaConfig struct {
	Brokers                  []string      `mapstructure:"brokers" toml:"brokers" json:"brokers"`
	ClientID                 string        `mapstructure:"client_id" toml:"client_id" json:"client_id"`
	Version                  string        `mapstructure:"version" toml:"version" json:"version"`
	ChannelBufferSize        int           `mapstructure:"channel_buffer_size" toml:"channel_buffer_size" json:"channel_buffer_size"`
	MaxOpenRequests          int           `mapstructure:"max_open_requests" toml:"max_open_requests" json:"max_open_requests"`
	DialTimeout              time.Duration `mapstructure:"dial_timeout" toml:"dial_timeout" json:"dial_timeout"`
	ReadTimeout              time.Duration `mapstructure:"read_timeout" toml:"read_timeout" json:"read_timeout"`
	WriteTimeout             time.Duration `mapstructure:"write_timeout" toml:"write_timeout" json:"write_timeout"`
	KeepAlive                time.Duration `mapstructure:"keepalive" toml:"keepalive" json:"keepalive"`
	MetadataRetryMax         int           `mapstructure:"metadata_retry_max" toml:"metadata_retry_max" json:"metadata_retry_max"`
	MetadataRetryBackoff     time.Duration `mapstructure:"metadata_retry_backoff" toml:"metadata_retry_backoff" json:"metadata_retry_backoff"`
	MetadataRefreshFrequency time.Duration `mapstructure:"metadata_refresh_frequency" toml:"metadata_refresh_frequency" json:"metadata_refresh_frequency"`
	MessageBytesMax          int           `mapstructure:"message_bytes_max" toml:"message_bytes_max" json:"message_bytes_max"`
	RequiredAcks             int16         `mapstructure:"required_acks" toml:"required_acks" json:"required_acks"`
	ProducerTimeout          time.Duration `mapstructure:"producer_timeout" toml:"producer_timeout" json:"producer_timeout"`
	Compression              string        `mapstructure:"compression" toml:"compression" json:"compression"`
	FlushBytes               int           `mapstructure:"flush_bytes" toml:"flush_bytes" json:"flush_bytes"`
	FlushMessages            int           `mapstructure:"flush_messages" toml:"flush_messages" json:"flush_messages"`
	FlushFrequency           time.Duration `mapstructure:"flush_frequency" toml:"flush_frequency" json:"flush_frequency"`
	FlushMessagesMax         int           `mapstructure:"flush_messages_max" toml:"flush_messages_max" json:"flush_messages_max"`
	RetrySendMax             int           `mapstructure:"retry_send_max" toml:"retry_send_max" json:"retry_send_max"`
	RetrySendBackoff         time.Duration `mapstructure:"retry_send_backoff" toml:"retry_send_backoff" json:"retry_send_backoff"`
	TLSEnabled               bool          `mapstructure:"tls_enabled" toml:"tls_enabled" json:"tls_enabled"`
	CAFile                   string        `mapstructure:"ca_file" toml:"ca_file" json:"ca_file"`
	CAPath                   string        `mapstructure:"ca_path" toml:"ca_path" json:"ca_path"`
	KeyFile                  string        `mapstructure:"key_file" toml:"key_file" json:"key_file"`
	CertFile                 string        `mapstructure:"cert_file" toml:"cert_file" json:"cert_file"`
	Insecure                 bool          `mapstructure:"insecure" toml:"insecure" json:"insecure"`
}

type JournaldConfig struct {
	Enabled       bool   `mapstructure:"enabled" toml:"enabled" json:"enabled"`
	TopicTmpl     string `mapstructure:"topic_tmpl" toml:"topic_tmpl" json:"topic_tmpl"`
	TopicFunc     string `mapstructure:"topic_function" toml:"topic_function" json:"topic_function"`
	PartitionTmpl string `mapstructure:"partition_key_tmpl" toml:"partition_key_tmpl" json:"partition_key_tmpl"`
	PartitionFunc string `mapstructure:"partition_key_func" toml:"partition_key_func" json:"partition_key_func"`
	FilterFunc    string `mapstructure:"filter_func" toml:"filter_func" json:"filter_func"`
	Encoding      string `mapstructure:"encoding" toml:"encoding" json:"encoding"`
	ConfID        string `mapstructure:"-" toml:"-" json:"conf_id"`
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
	TLSEnabled      bool          `mapstructure:"tls_enabled" toml:"tls_enabled" json:"tls_enabled"`
	CAFile          string        `mapstructure:"ca_file" toml:"ca_file" json:"ca_file"`
	CAPath          string        `mapstructure:"ca_path" toml:"ca_path" json:"ca_path"`
	KeyFile         string        `mapstructure:"key_file" toml:"key_file" json:"key_file"`
	CertFile        string        `mapstructure:"cert_file" toml:"cert_file" json:"cert_file"`
	ClientAuthType  string        `mapstructure:"client_auth_type" toml:"client_auth_type" json:"client_auth_type"`
	Encoding        string        `mapstructure:"encoding" toml:"encoding" json:"encoding"`
	ConfID          string        `mapstructure:"-" toml:"-" json:"conf_id"`
	// todo: Partitioner ?
}

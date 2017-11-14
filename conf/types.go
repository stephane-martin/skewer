package conf

import (
	"encoding/base64"
	"time"

	"github.com/oklog/ulid"
)

// DestinationType lists the possible kind of destinations where skewer can forward messages.
type DestinationType uint64

const (
	Kafka  DestinationType = 1
	Udp                    = 2
	Tcp                    = 4
	Relp                   = 8
	File                   = 16
	Stderr                 = 32
)

var Destinations = map[string]DestinationType{
	"kafka":  Kafka,
	"udp":    Udp,
	"tcp":    Tcp,
	"relp":   Relp,
	"file":   File,
	"stderr": Stderr,
}

var RDestinations = map[DestinationType]byte{
	Kafka:  'k',
	Udp:    'u',
	Tcp:    't',
	Relp:   'r',
	File:   'f',
	Stderr: 's',
}

// BaseConfig is the root of all configuration parameters.
type BaseConfig struct {
	Syslog     []SyslogConfig   `mapstructure:"syslog" toml:"syslog" json:"syslog"`
	Store      StoreConfig      `mapstructure:"store" toml:"store" json:"store"`
	Parsers    []ParserConfig   `mapstructure:"parser" toml:"parser" json:"parser"`
	Journald   JournaldConfig   `mapstructure:"journald" toml:"journald" json:"journald"`
	Metrics    MetricsConfig    `mapstructure:"metrics" toml:"metrics" json:"metrics"`
	Accounting AccountingConfig `mapstructure:"accounting" toml:"accounting" json:"accounting"`
	Main       MainConfig       `mapstructure:"main" toml:"main" json:"main"`
	KafkaDest  KafkaDestConfig  `mapstructure:"kafka_destination" toml:"kafka_destination" json:"kafka_destination"`
	UdpDest    UdpDestConfig    `mapstructure:"udp_destination" toml:"udp_destination" json:"udp_destination"`
	TcpDest    TcpDestConfig    `mapstructure:"tcp_destination" toml:"tcp_destination" json:"tcp_destination"`
	RelpDest   RelpDestConfig   `mapstructure:"relp_destination" toml:"relp_destination" json:"relp_destination"`
	FileDest   FileDestConfig   `mapstructure:"file_destination" toml:"file_destination" json:"file_destination"`
	StderrDest StderrDestConfig `mapstructure:"stderr_destination" toml:"stderr_destination" json:"stderr_destination"`
}

// MainConfig lists general/global parameters.
type MainConfig struct {
	DirectRelp          bool            `mapstructure:"direct_relp" toml:"direct_relp" json:"direct_relp"`
	InputQueueSize      uint64          `mapstructure:"input_queue_size" toml:"input_queue_size" json:"input_queue_size"`
	MaxInputMessageSize int             `mapstructure:"max_input_message_size" toml:"max_input_message_size" json:"max_input_message_size"`
	Destination         string          `mapstructure:"destination" toml:"destination" json:"destination"`
	Destinations        DestinationType `mapstructure:"-" toml:"-" json:"dest"`
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
	Dirname string `mapstructure:"-" toml:"-" json:"dirname"`
	Maxsize int64  `mapstructure:"max_size" toml:"max_size" json:"max_size"`
	FSync   bool   `mapstructure:"fsync" toml:"fsync" json:"fsync"`
	Secret  string `mapstructure:"secret" toml:"-" json:"secret"`
}

func (s *StoreConfig) GetSecretB() (secretb [32]byte, err error) {
	if len(s.Secret) == 0 {
		return
	}
	var n int
	t := make([]byte, base64.URLEncoding.DecodedLen(len(s.Secret)))
	n, err = base64.URLEncoding.Decode(t, []byte(s.Secret))
	if err != nil {
		return secretb, ConfigurationCheckError{ErrString: "Error decoding store secret", Err: err}
	}
	if n < 32 {
		return secretb, ConfigurationCheckError{ErrString: "Store secret is too short"}
	}
	copy(secretb[:], t[:32])
	return secretb, nil
}

type BaseDestConfig struct {
	Format string `mapstructure:"format" toml:"format" json:"format"`
}

type KafkaDestConfig struct {
	TlsBaseConfig            `mapstructure:",squash"`
	BaseDestConfig           `mapstructure:",squash"`
	Insecure                 bool          `mapstructure:"insecure" toml:"insecure" json:"insecure"`
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
	Partitioner              string        `mapstructure:"partitioner" toml:"partitioner" json:"partitioner"`
}

type TcpUdpRelpDestBaseConfig struct {
	BaseDestConfig `mapstructure:",squash"`
	Host           string        `mapstructure:"host" toml:"host" json:"host"`
	Port           int           `mapstructure:"port" toml:"port" json:"port"`
	UnixSocketPath string        `mapstructure:"unix_socket_path" toml:"unix_socket_path" json:"unix_socket_path"`
	Rebind         time.Duration `mapstructure:"rebind" toml:"rebind" json:"rebind"`
}

type UdpDestConfig struct {
	TcpUdpRelpDestBaseConfig `mapstructure:",squash"`
}

type RelpDestConfig struct {
	TcpUdpRelpDestBaseConfig `mapstructure:",squash"`
	KeepAlive                bool          `mapstructure:"keepalive" toml:"keepalive" json:"keepalive"`
	KeepAlivePeriod          time.Duration `mapstructure:"keepalive_period" toml:"keepalive_period" json:"keepalive_period"`
	WindowSize               int           `mapstructure:"window_size" toml:"window_size" json:"window_size"`
	ConnTimeout              time.Duration `mapstructure:"connection_timeout" toml:"connection_timeout" json:"connection_timeout"`
	RelpTimeout              time.Duration `mapstructure:"relp_timeout" toml:"relp_timeout" json:"relp_timeout"`
}

type TcpDestConfig struct {
	TcpUdpRelpDestBaseConfig `mapstructure:",squash"`
	LineFraming              bool          `mapstructure:"line_framing" toml:"line_framing" json:"line_framing"`
	FrameDelimiter           uint8         `mapstructure:"delimiter" toml:"delimiter" json:"delimiter"`
	KeepAlive                bool          `mapstructure:"keepalive" toml:"keepalive" json:"keepalive"`
	KeepAlivePeriod          time.Duration `mapstructure:"keepalive_period" toml:"keepalive_period" json:"keepalive_period"`
}

type FileDestConfig struct {
	BaseDestConfig  `mapstructure:",squash"`
	Filename        string        `mapstructure:"filename" toml:"filename" json:"filename"`
	Sync            bool          `mapstructure:"sync" toml:"sync" json:"sync"`
	SyncPeriod      time.Duration `mapstructure:"sync_period" toml:"sync_period" json:"sync_period"`
	OpenFilesCache  uint64        `mapstructure:"open_files_cache" toml:"open_files_cache" json:"open_files_cache"`
	OpenFileTimeout time.Duration `mapstructure:"open_file_timeout" toml:"open_file_timeout" json:"open_file_timeout"`
	Gzip            bool          `mapstructure:"gzip" toml:"gzip" json:"gzip"`
	GzipLevel       int           `mapstructure:"gzip_level" toml:"gzip_level" json:"gzip_level"`
}

type StderrDestConfig struct {
	BaseDestConfig `mapstructure:",squash"`
}

type FilterSubConfig struct {
	TopicTmpl           string `mapstructure:"topic_tmpl" toml:"topic_tmpl" json:"topic_tmpl"`
	TopicFunc           string `mapstructure:"topic_function" toml:"topic_function" json:"topic_function"`
	PartitionTmpl       string `mapstructure:"partition_key_tmpl" toml:"partition_key_tmpl" json:"partition_key_tmpl"`
	PartitionFunc       string `mapstructure:"partition_key_func" toml:"partition_key_func" json:"partition_key_func"`
	PartitionNumberFunc string `mapstructure:"partition_number_func" toml:"partition_number_func" json:"partition_number_func"`
	FilterFunc          string `mapstructure:"filter_func" toml:"filter_func" json:"filter_func"`
	Encoding            string `mapstructure:"encoding" toml:"encoding" json:"encoding"`
}

type JournaldConfig struct {
	FilterSubConfig `mapstructure:",squash"`
	ConfID          ulid.ULID `mapstructure:"-" toml:"-" json:"conf_id"`
	Enabled         bool      `mapstructure:"enabled" toml:"enabled" json:"enabled"`
}

type AccountingConfig struct {
	FilterSubConfig `mapstructure:",squash"`
	ConfID          ulid.ULID     `mapstructure:"-" toml:"-" json:"conf_id"`
	Period          time.Duration `mapstructure:"period" toml:"period" json:"period"`
	Path            string        `mapstructure:"path" toml:"path" json:"path"`
	Enabled         bool          `mapstructure:"enabled" toml:"enabled" json:"enabled"`
}

type SyslogConfig struct {
	FilterSubConfig `mapstructure:",squash"`
	TlsBaseConfig   `mapstructure:",squash"`
	ClientAuthType  string        `mapstructure:"client_auth_type" toml:"client_auth_type" json:"client_auth_type"`
	ConfID          ulid.ULID     `mapstructure:"-" toml:"-" json:"conf_id"`
	Port            int           `mapstructure:"port" toml:"port" json:"port"`
	BindAddr        string        `mapstructure:"bind_addr" toml:"bind_addr" json:"bind_addr"`
	UnixSocketPath  string        `mapstructure:"unix_socket_path" toml:"unix_socket_path" json:"unix_socket_path"`
	Format          string        `mapstructure:"format" toml:"format" json:"format"`
	Protocol        string        `mapstructure:"protocol" toml:"protocol" json:"protocol"`
	DontParseSD     bool          `mapstructure:"dont_parse_structured_data" toml:"dont_parse_structured_data" json:"dont_parse_structured_data"`
	KeepAlive       bool          `mapstructure:"keepalive" toml:"keepalive" json:"keepalive"`
	KeepAlivePeriod time.Duration `mapstructure:"keepalive_period" toml:"keepalive_period" json:"keepalive_period"`
	Timeout         time.Duration `mapstructure:"timeout" toml:"timeout" json:"timeout"`
	Encoding        string        `mapstructure:"encoding" toml:"encoding" json:"encoding"`
}

type TlsBaseConfig struct {
	TLSEnabled bool   `mapstructure:"tls_enabled" toml:"tls_enabled" json:"tls_enabled"`
	CAFile     string `mapstructure:"ca_file" toml:"ca_file" json:"ca_file"`
	CAPath     string `mapstructure:"ca_path" toml:"ca_path" json:"ca_path"`
	KeyFile    string `mapstructure:"key_file" toml:"key_file" json:"key_file"`
	CertFile   string `mapstructure:"cert_file" toml:"cert_file" json:"cert_file"`
}

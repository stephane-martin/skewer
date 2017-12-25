package conf

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/awnumar/memguard"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/skewer/utils/sbox"
)

// DestinationType lists the possible kind of destinations where skewer can forward messages.
type DestinationType uint64

func (dests DestinationType) Iterate() (res []DestinationType) {
	if dests == 0 {
		return []DestinationType{Stderr}
	}
	res = make([]DestinationType, 0, len(Destinations))
	for _, dtype := range Destinations {
		if uint64(dests)&uint64(dtype) != 0 {
			res = append(res, dtype)
		}
	}
	return res
}

const (
	Kafka   DestinationType = 1
	Udp                     = 2
	Tcp                     = 4
	Relp                    = 8
	File                    = 16
	Stderr                  = 32
	Graylog                 = 64
)

var Destinations = map[string]DestinationType{
	"kafka":   Kafka,
	"udp":     Udp,
	"tcp":     Tcp,
	"relp":    Relp,
	"file":    File,
	"stderr":  Stderr,
	"graylog": Graylog,
}

var DestinationNames = map[DestinationType]string{
	Kafka:   "kafka",
	Udp:     "udp",
	Tcp:     "tcp",
	Relp:    "relp",
	File:    "file",
	Stderr:  "stderr",
	Graylog: "graylog",
}

var RDestinations = map[DestinationType]byte{
	Kafka:   'k',
	Udp:     'u',
	Tcp:     't',
	Relp:    'r',
	File:    'f',
	Stderr:  's',
	Graylog: 'g',
}

// BaseConfig is the root of all configuration parameters.
type BaseConfig struct {
	TcpSource   []TcpSourceConfig   `mapstructure:"tcp_source" toml:"tcp_source" json:"tcp_source"`
	UdpSource   []UdpSourceConfig   `mapstructure:"udp_source" toml:"udp_source" json:"udp_source"`
	RelpSource  []RelpSourceConfig  `mapstructure:"relp_source" toml:"relp_source" json:"relp_source"`
	KafkaSource []KafkaSourceConfig `mapstructure:"kafka_source" toml:"kafka_source" json:"kafka_source"`
	Store       StoreConfig         `mapstructure:"store" toml:"store" json:"store"`
	Parsers     []ParserConfig      `mapstructure:"parser" toml:"parser" json:"parser"`
	Journald    JournaldConfig      `mapstructure:"journald" toml:"journald" json:"journald"`
	Metrics     MetricsConfig       `mapstructure:"metrics" toml:"metrics" json:"metrics"`
	Accounting  AccountingConfig    `mapstructure:"accounting" toml:"accounting" json:"accounting"`
	Main        MainConfig          `mapstructure:"main" toml:"main" json:"main"`
	KafkaDest   KafkaDestConfig     `mapstructure:"kafka_destination" toml:"kafka_destination" json:"kafka_destination"`
	UdpDest     UdpDestConfig       `mapstructure:"udp_destination" toml:"udp_destination" json:"udp_destination"`
	TcpDest     TcpDestConfig       `mapstructure:"tcp_destination" toml:"tcp_destination" json:"tcp_destination"`
	RelpDest    RelpDestConfig      `mapstructure:"relp_destination" toml:"relp_destination" json:"relp_destination"`
	FileDest    FileDestConfig      `mapstructure:"file_destination" toml:"file_destination" json:"file_destination"`
	StderrDest  StderrDestConfig    `mapstructure:"stderr_destination" toml:"stderr_destination" json:"stderr_destination"`
	GraylogDest GraylogDestConfig   `mapstructure:"graylog_destination" toml:"graylog_destination" json:"graylog_destination"`
}

// MainConfig lists general/global parameters.
type MainConfig struct {
	DirectRelp          bool   `mapstructure:"direct_relp" toml:"direct_relp" json:"direct_relp"`
	InputQueueSize      uint64 `mapstructure:"input_queue_size" toml:"input_queue_size" json:"input_queue_size"`
	MaxInputMessageSize int    `mapstructure:"max_input_message_size" toml:"max_input_message_size" json:"max_input_message_size"`
	Destination         string `mapstructure:"destination" toml:"destination" json:"destination"`
	EncryptIPC          bool   `mapstructure:"encrypt_ipc" toml:"encrypt_ipc" json:"encrypt_ipc"`
}

func (m *MainConfig) GetDestinations() (dests DestinationType, err error) {
	destr := strings.TrimSpace(strings.ToLower(m.Destination))
	for _, dest := range strings.Split(destr, ",") {
		d, ok := Destinations[strings.TrimSpace(dest)]
		if ok {
			dests = dests | d
		} else {
			return 0, ConfigurationCheckError{ErrString: fmt.Sprintf("Unknown destination type: '%s'", dest)}
		}
	}
	if dests == 0 {
		return Stderr, nil
	}
	return
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
	Dirname   string `mapstructure:"-" toml:"-" json:"dirname"`
	Maxsize   int64  `mapstructure:"max_size" toml:"max_size" json:"max_size"`
	FSync     bool   `mapstructure:"fsync" toml:"fsync" json:"fsync"`
	Secret    string `mapstructure:"secret" toml:"-" json:"secret"`
	BatchSize uint32 `mapstructure:"batch_size" toml:"batch_size" json:"batch_size"`
}

// the Secret in StoreConfig will be encrypted with the session secret in Complete()
// so we do not transport an unencrypted secret between the multiple skewer processes

func (s *StoreConfig) GetSecretB(m *memguard.LockedBuffer) (secretb *memguard.LockedBuffer, err error) {
	locked, err := s.DecryptSecret(m)
	if err != nil {
		return nil, err
	}
	if locked == nil {
		return nil, nil
	}
	defer locked.Destroy()

	var n int = base64.URLEncoding.DecodedLen(len(locked.Buffer()))
	if n < 32 {
		return nil, ConfigurationCheckError{ErrString: "Store secret is too short"}
	}
	secret := make([]byte, n)
	n, err = base64.URLEncoding.Decode(secret, locked.Buffer())
	if err != nil {
		return nil, ConfigurationCheckError{ErrString: "Error decoding store secret", Err: err}
	}
	if n < 32 {
		return nil, ConfigurationCheckError{ErrString: "Store secret is too short"}
	}
	secret = secret[:32]
	secretb, err = memguard.NewImmutableFromBytes(secret)
	if err != nil {
		return nil, err
	}
	return secretb, nil
}

func (s *StoreConfig) EncryptSecret(m *memguard.LockedBuffer) error {
	secret := strings.TrimSpace(s.Secret)
	if len(secret) == 0 {
		s.Secret = ""
		return nil
	}
	enc, err := sbox.Encrypt([]byte(secret), m)
	if err != nil {
		s.Secret = ""
		return err
	}
	s.Secret = base64.StdEncoding.EncodeToString(enc)
	return nil
}

func (s *StoreConfig) DecryptSecret(m *memguard.LockedBuffer) (locked *memguard.LockedBuffer, err error) {
	if len(s.Secret) == 0 {
		return nil, nil
	}
	enc, err := base64.StdEncoding.DecodeString(s.Secret)
	if err != nil {
		return nil, err
	}
	dec, err := sbox.Decrypt(enc, m)
	if err != nil {
		return nil, err
	}
	locked, err = memguard.NewImmutableFromBytes(dec)
	if err != nil {
		return nil, err
	}
	return locked, nil
}

type KafkaDestConfig struct {
	KafkaBaseConfig         `mapstructure:",squash"`
	KafkaProducerBaseConfig `mapstructure:",squash"`
	TlsBaseConfig           `mapstructure:",squash"`
	Insecure                bool   `mapstructure:"insecure" toml:"insecure" json:"insecure"`
	Format                  string `mapstructure:"format" toml:"format" json:"format"`
}

type KafkaBaseConfig struct {
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
}

type KafkaConsumerBaseConfig struct {
	RetryBackoff          time.Duration `mapstructure:"retry_backoff" toml:"retry_backoff" json:"retry_backoff"`
	MinFetchBytes         int32         `mapstructure:"min_fetch_bytes" toml:"min_fetch_bytes" json:"min_fetch_bytes"`
	DefaultFetchBytes     int32         `mapstructure:"default_fetch_bytes" toml:"default_fetch_bytes" json:"default_fetch_bytes"`
	MaxFetchBytes         int32         `mapstructure:"max_fetch_bytes" toml:"max_fetch_bytes" json:"max_fetch_bytes"`
	MaxWaitTime           time.Duration `mapstructure:"max_wait_time" toml:"max_wait_time" json:"max_wait_time"`
	MaxProcessingTime     time.Duration `mapstructure:"max_processing_time" toml:"max_processing_time" json:"max_processing_time"`
	OffsetsCommitInterval time.Duration `mapstructure:"offsets_commit_interval" toml:"offsets_commit_interval" json:"offsets_commit_interval"`
	OffsetsInitial        int64         `mapstructure:"offsets_initial" toml:"offsets_initial" json:"offsets_initial"`
	OffsetsRetention      time.Duration `mapstructure:"offsets_retention" toml:"offsets_retention" json:"offsets_retention"`
}

type KafkaProducerBaseConfig struct {
	MessageBytesMax  int           `mapstructure:"message_bytes_max" toml:"message_bytes_max" json:"message_bytes_max"`
	RequiredAcks     int16         `mapstructure:"required_acks" toml:"required_acks" json:"required_acks"`
	ProducerTimeout  time.Duration `mapstructure:"producer_timeout" toml:"producer_timeout" json:"producer_timeout"`
	Compression      string        `mapstructure:"compression" toml:"compression" json:"compression"`
	Partitioner      string        `mapstructure:"partitioner" toml:"partitioner" json:"partitioner"`
	FlushBytes       int           `mapstructure:"flush_bytes" toml:"flush_bytes" json:"flush_bytes"`
	FlushMessages    int           `mapstructure:"flush_messages" toml:"flush_messages" json:"flush_messages"`
	FlushFrequency   time.Duration `mapstructure:"flush_frequency" toml:"flush_frequency" json:"flush_frequency"`
	FlushMessagesMax int           `mapstructure:"flush_messages_max" toml:"flush_messages_max" json:"flush_messages_max"`
	RetrySendMax     int           `mapstructure:"retry_send_max" toml:"retry_send_max" json:"retry_send_max"`
	RetrySendBackoff time.Duration `mapstructure:"retry_send_backoff" toml:"retry_send_backoff" json:"retry_send_backoff"`
}

type GraylogDestConfig struct {
	Host             string        `mapstructure:"host" toml:"host" json:"host"`
	Port             int           `mapstructure:"port" toml:"port" json:"port"`
	Mode             string        `mapstructure:"mode" toml:"mode" json:"mode"`
	MaxReconnect     int           `mapstructure:"max_reconnect" toml:"max_reconnect" json:"max_reconnect"`
	ReconnectDelay   time.Duration `mapstructure:"reconnect_delay" toml:"reconnect_delay" json:"reconnect_delay"`
	CompressionLevel int           `mapstructure:"compression_level" toml:"compression_level" json:"compression_level"`
	CompressionType  string        `mapstructure:"compression_type" toml:"compression_type" json:"compression_type"`
}

type TcpUdpRelpDestBaseConfig struct {
	Host           string        `mapstructure:"host" toml:"host" json:"host"`
	Port           int           `mapstructure:"port" toml:"port" json:"port"`
	UnixSocketPath string        `mapstructure:"unix_socket_path" toml:"unix_socket_path" json:"unix_socket_path"`
	Rebind         time.Duration `mapstructure:"rebind" toml:"rebind" json:"rebind"`
	Format         string        `mapstructure:"format" toml:"format" json:"format"`
}

type UdpDestConfig struct {
	TcpUdpRelpDestBaseConfig `mapstructure:",squash"`
}

type RelpDestConfig struct {
	TcpUdpRelpDestBaseConfig `mapstructure:",squash"`
	TlsBaseConfig            `mapstructure:",squash"`
	Insecure                 bool          `mapstructure:"insecure" toml:"insecure" json:"insecure"`
	KeepAlive                bool          `mapstructure:"keepalive" toml:"keepalive" json:"keepalive"`
	KeepAlivePeriod          time.Duration `mapstructure:"keepalive_period" toml:"keepalive_period" json:"keepalive_period"`
	ConnTimeout              time.Duration `mapstructure:"connection_timeout" toml:"connection_timeout" json:"connection_timeout"`
	FlushPeriod              time.Duration `mapstructure:"flush_period" toml:"flush_period" json:"flush_period"`

	WindowSize  int32         `mapstructure:"window_size" toml:"window_size" json:"window_size"`
	RelpTimeout time.Duration `mapstructure:"relp_timeout" toml:"relp_timeout" json:"relp_timeout"`
}

type TcpDestConfig struct {
	TcpUdpRelpDestBaseConfig `mapstructure:",squash"`
	TlsBaseConfig            `mapstructure:",squash"`
	Insecure                 bool          `mapstructure:"insecure" toml:"insecure" json:"insecure"`
	KeepAlive                bool          `mapstructure:"keepalive" toml:"keepalive" json:"keepalive"`
	KeepAlivePeriod          time.Duration `mapstructure:"keepalive_period" toml:"keepalive_period" json:"keepalive_period"`
	ConnTimeout              time.Duration `mapstructure:"connection_timeout" toml:"connection_timeout" json:"connection_timeout"`
	FlushPeriod              time.Duration `mapstructure:"flush_period" toml:"flush_period" json:"flush_period"`

	LineFraming    bool  `mapstructure:"line_framing" toml:"line_framing" json:"line_framing"`
	FrameDelimiter uint8 `mapstructure:"delimiter" toml:"delimiter" json:"delimiter"`
}

type FileDestConfig struct {
	Filename        string        `mapstructure:"filename" toml:"filename" json:"filename"`
	Sync            bool          `mapstructure:"sync" toml:"sync" json:"sync"`
	SyncPeriod      time.Duration `mapstructure:"sync_period" toml:"sync_period" json:"sync_period"`
	FlushPeriod     time.Duration `mapstructure:"flush_period" toml:"flush_period" json:"flush_period"`
	BufferSize      int           `mapstructure:"buffer_size" toml:"buffer_size" json:"buffer_size"`
	OpenFilesCache  uint64        `mapstructure:"open_files_cache" toml:"open_files_cache" json:"open_files_cache"`
	OpenFileTimeout time.Duration `mapstructure:"open_file_timeout" toml:"open_file_timeout" json:"open_file_timeout"`
	Gzip            bool          `mapstructure:"gzip" toml:"gzip" json:"gzip"`
	GzipLevel       int           `mapstructure:"gzip_level" toml:"gzip_level" json:"gzip_level"`
	Format          string        `mapstructure:"format" toml:"format" json:"format"`
}

type StderrDestConfig struct {
	Format string `mapstructure:"format" toml:"format" json:"format"`
}

type FilterSubConfig struct {
	TopicTmpl           string `mapstructure:"topic_tmpl" toml:"topic_tmpl" json:"topic_tmpl"`
	TopicFunc           string `mapstructure:"topic_function" toml:"topic_function" json:"topic_function"`
	PartitionTmpl       string `mapstructure:"partition_key_tmpl" toml:"partition_key_tmpl" json:"partition_key_tmpl"`
	PartitionFunc       string `mapstructure:"partition_key_func" toml:"partition_key_func" json:"partition_key_func"`
	PartitionNumberFunc string `mapstructure:"partition_number_func" toml:"partition_number_func" json:"partition_number_func"`
	FilterFunc          string `mapstructure:"filter_func" toml:"filter_func" json:"filter_func"`
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

type TcpSourceConfig struct {
	SyslogSourceBaseConfig `mapstructure:",squash"`
	FilterSubConfig        `mapstructure:",squash"`
	TlsBaseConfig          `mapstructure:",squash"`
	ClientAuthType         string    `mapstructure:"client_auth_type" toml:"client_auth_type" json:"client_auth_type"`
	LineFraming            bool      `mapstructure:"line_framing" toml:"line_framing" json:"line_framing"`
	FrameDelimiter         string    `mapstructure:"delimiter" toml:"delimiter" json:"delimiter"`
	ConfID                 ulid.ULID `mapstructure:"-" toml:"-" json:"conf_id"`
}

func (c *TcpSourceConfig) GetFilterConf() *FilterSubConfig {
	return &c.FilterSubConfig
}

func (c *TcpSourceConfig) GetSyslogConf() *SyslogSourceBaseConfig {
	return &c.SyslogSourceBaseConfig
}

func (c *TcpSourceConfig) DefaultPort() int {
	return 1514
}

type UdpSourceConfig struct {
	SyslogSourceBaseConfig `mapstructure:",squash"`
	FilterSubConfig        `mapstructure:",squash"`
	ConfID                 ulid.ULID `mapstructure:"-" toml:"-" json:"conf_id"`
}

func (c *UdpSourceConfig) GetFilterConf() *FilterSubConfig {
	return &c.FilterSubConfig
}

func (c *UdpSourceConfig) GetSyslogConf() *SyslogSourceBaseConfig {
	return &c.SyslogSourceBaseConfig
}

func (c *UdpSourceConfig) DefaultPort() int {
	return 1514
}

type RelpSourceConfig struct {
	SyslogSourceBaseConfig `mapstructure:",squash"`
	FilterSubConfig        `mapstructure:",squash"`
	TlsBaseConfig          `mapstructure:",squash"`
	ClientAuthType         string    `mapstructure:"client_auth_type" toml:"client_auth_type" json:"client_auth_type"`
	LineFraming            bool      `mapstructure:"line_framing" toml:"line_framing" json:"line_framing"`
	FrameDelimiter         string    `mapstructure:"delimiter" toml:"delimiter" json:"delimiter"`
	ConfID                 ulid.ULID `mapstructure:"-" toml:"-" json:"conf_id"`
}

func (c *RelpSourceConfig) GetFilterConf() *FilterSubConfig {
	return &c.FilterSubConfig
}

func (c *RelpSourceConfig) GetSyslogConf() *SyslogSourceBaseConfig {
	return &c.SyslogSourceBaseConfig
}

func (c *RelpSourceConfig) DefaultPort() int {
	return 2514
}

type SyslogSourceConfig interface {
	GetFilterConf() *FilterSubConfig
	GetSyslogConf() *SyslogSourceBaseConfig
	DefaultPort() int
	SetConfID()
}

type SyslogSourceBaseConfig struct {
	Ports           []int         `mapstructure:"ports" toml:"ports" json:"ports"`
	BindAddr        string        `mapstructure:"bind_addr" toml:"bind_addr" json:"bind_addr"`
	UnixSocketPath  string        `mapstructure:"unix_socket_path" toml:"unix_socket_path" json:"unix_socket_path"`
	Format          string        `mapstructure:"format" toml:"format" json:"format"`
	DontParseSD     bool          `mapstructure:"dont_parse_structured_data" toml:"dont_parse_structured_data" json:"dont_parse_structured_data"`
	KeepAlive       bool          `mapstructure:"keepalive" toml:"keepalive" json:"keepalive"`
	KeepAlivePeriod time.Duration `mapstructure:"keepalive_period" toml:"keepalive_period" json:"keepalive_period"`
	Timeout         time.Duration `mapstructure:"timeout" toml:"timeout" json:"timeout"`
	Encoding        string        `mapstructure:"encoding" toml:"encoding" json:"encoding"`
}

type KafkaSourceConfig struct {
	KafkaBaseConfig         `mapstructure:",squash"`
	KafkaConsumerBaseConfig `mapstructure:",squash"`
	FilterSubConfig         `mapstructure:",squash"`
	TlsBaseConfig           `mapstructure:",squash"`
	Insecure                bool          `mapstructure:"insecure" toml:"insecure" json:"insecure"`
	Format                  string        `mapstructure:"format" toml:"format" json:"format"`
	Encoding                string        `mapstructure:"encoding" toml:"encoding" json:"encoding"`
	ConfID                  ulid.ULID     `mapstructure:"-" toml:"-" json:"conf_id"`
	SessionTimeout          time.Duration `mapstructure:"session_timeout" toml:"session_timeout" json:"session_timeout"`
	HeartbeatInterval       time.Duration `mapstructure:"heartbeat_interval" toml:"heartbeat_interval" json:"heartbeat_interval"`
	OffsetsMaxRetry         int           `mapstructure:"offsets_max_retry" toml:"offsets_max_retry" json:"offsets_max_retry"`
	GroupID                 string        `mapstructure:"group_ip" toml:"group_id" json:"group_id"`
	Topics                  []string      `mapstructure:"topics" toml:"topics" json:"topics"`
}

type TlsBaseConfig struct {
	TLSEnabled bool   `mapstructure:"tls_enabled" toml:"tls_enabled" json:"tls_enabled"`
	CAFile     string `mapstructure:"ca_file" toml:"ca_file" json:"ca_file"`
	CAPath     string `mapstructure:"ca_path" toml:"ca_path" json:"ca_path"`
	KeyFile    string `mapstructure:"key_file" toml:"key_file" json:"key_file"`
	CertFile   string `mapstructure:"cert_file" toml:"cert_file" json:"cert_file"`
}

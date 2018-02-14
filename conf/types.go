package conf

import (
	"encoding/base64"
	"strings"
	"time"

	"github.com/awnumar/memguard"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/sbox"
)

// BaseConfig is the root of all configuration parameters.
type BaseConfig struct {
	FSSource            []FilesystemSourceConfig  `mapstructure:"fs_source" toml:"fs_source" json:"fs_source"`
	TCPSource           []TCPSourceConfig         `mapstructure:"tcp_source" toml:"tcp_source" json:"tcp_source"`
	UDPSource           []UDPSourceConfig         `mapstructure:"udp_source" toml:"udp_source" json:"udp_source"`
	RELPSource          []RELPSourceConfig        `mapstructure:"relp_source" toml:"relp_source" json:"relp_source"`
	HTTPServerSource    []HTTPServerSourceConfig  `mapstructure:"httpserver_source" toml:"httpserver_source" json:"httpserver_source"`
	DirectRELPSource    []DirectRELPSourceConfig  `mapstructure:"directrelp_source" toml:"directrelp_source" json:"directrelp_source"`
	KafkaSource         []KafkaSourceConfig       `mapstructure:"kafka_source" toml:"kafka_source" json:"kafka_source"`
	GraylogSource       []GraylogSourceConfig     `mapstructure:"graylog_source" toml:"graylog_source" json:"graylog_source"`
	Store               StoreConfig               `mapstructure:"store" toml:"store" json:"store"`
	Parsers             []ParserConfig            `mapstructure:"parser" toml:"parser" json:"parser"`
	Journald            JournaldConfig            `mapstructure:"journald" toml:"journald" json:"journald"`
	Metrics             MetricsConfig             `mapstructure:"metrics" toml:"metrics" json:"metrics"`
	Accounting          AccountingSourceConfig    `mapstructure:"accounting" toml:"accounting" json:"accounting"`
	MacOS               MacOSSourceConfig         `mapstructure:"macos" toml:"macos" json:"macos"`
	Main                MainConfig                `mapstructure:"main" toml:"main" json:"main"`
	KafkaDest           *KafkaDestConfig          `mapstructure:"kafka_destination" toml:"kafka_destination" json:"kafka_destination"`
	UDPDest             UDPDestConfig             `mapstructure:"udp_destination" toml:"udp_destination" json:"udp_destination"`
	TCPDest             TCPDestConfig             `mapstructure:"tcp_destination" toml:"tcp_destination" json:"tcp_destination"`
	HTTPDest            HTTPDestConfig            `mapstructure:"http_destination" toml:"http_destination" json:"http_destination"`
	HTTPServerDest      HTTPServerDestConfig      `mapstructure:"httpserver_destination" toml:"httpserver_destination" json:"httpserver_destination"`
	WebsocketServerDest WebsocketServerDestConfig `mapstructure:"websocketserver_destination" toml:"websocketserver_destination" json:"websocketserver_destination"`
	NATSDest            *NATSDestConfig           `mapstructure:"nats_destination" toml:"nats_destination" json:"nats_destination"`
	RELPDest            RELPDestConfig            `mapstructure:"relp_destination" toml:"relp_destination" json:"relp_destination"`
	FileDest            FileDestConfig            `mapstructure:"file_destination" toml:"file_destination" json:"file_destination"`
	StderrDest          StderrDestConfig          `mapstructure:"stderr_destination" toml:"stderr_destination" json:"stderr_destination"`
	GraylogDest         GraylogDestConfig         `mapstructure:"graylog_destination" toml:"graylog_destination" json:"graylog_destination"`
	ElasticDest         ElasticDestConfig         `mapstructure:"elasticsearch_destination" toml:"elasticsearch_destination" json:"elasticsearch_destination"`
	RedisDest           RedisDestConfig           `mapstructure:"redis_destination" toml:"redis_destination" json:"redis_destination"`
}

// MainConfig lists general/global parameters.
type MainConfig struct {
	InputQueueSize      uint64 `mapstructure:"input_queue_size" toml:"input_queue_size" json:"input_queue_size"`
	MaxInputMessageSize int    `mapstructure:"max_input_message_size" toml:"max_input_message_size" json:"max_input_message_size"`
	Destination         string `mapstructure:"destination" toml:"destination" json:"destination"`
	EncryptIPC          bool   `mapstructure:"encrypt_ipc" toml:"encrypt_ipc" json:"encrypt_ipc"`
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
	Dirname          string `mapstructure:"-" toml:"-" json:"dirname"`
	MaxTableSize     int64  `mapstructure:"max_table_size" toml:"max_table_size" json:"max_table_size"`
	ValueLogFileSize int64  `mapstructure:"value_log_file_size" toml:"value_log_file_size" json:"value_log_file_size"`
	FSync            bool   `mapstructure:"fsync" toml:"fsync" json:"fsync"`
	Secret           string `mapstructure:"secret" toml:"-" json:"secret"`
	BatchSize        uint32 `mapstructure:"batch_size" toml:"batch_size" json:"batch_size"`
	AddMissingMsgID  bool   `mapstructure:"add_missing_msgid" toml:"add_missing_msgid" json:"add_missing_msgid"`
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
	Insecure bool           `mapstructure:"insecure" toml:"insecure" json:"insecure"`
	Format   string         `mapstructure:"format" toml:"format" json:"format"`
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

type UDPDestConfig struct {
	TcpUdpRelpDestBaseConfig `mapstructure:",squash"`
}

type RELPDestConfig struct {
	TcpUdpRelpDestBaseConfig      `mapstructure:",squash"`
	TlsBaseConfig                 `mapstructure:",squash"`
	Insecure        bool          `mapstructure:"insecure" toml:"insecure" json:"insecure"`
	KeepAlive       bool          `mapstructure:"keepalive" toml:"keepalive" json:"keepalive"`
	KeepAlivePeriod time.Duration `mapstructure:"keepalive_period" toml:"keepalive_period" json:"keepalive_period"`
	ConnTimeout     time.Duration `mapstructure:"connection_timeout" toml:"connection_timeout" json:"connection_timeout"`
	FlushPeriod     time.Duration `mapstructure:"flush_period" toml:"flush_period" json:"flush_period"`

	WindowSize  int32         `mapstructure:"window_size" toml:"window_size" json:"window_size"`
	RelpTimeout time.Duration `mapstructure:"relp_timeout" toml:"relp_timeout" json:"relp_timeout"`
}

type TCPDestConfig struct {
	TcpUdpRelpDestBaseConfig      `mapstructure:",squash"`
	TlsBaseConfig                 `mapstructure:",squash"`
	Insecure        bool          `mapstructure:"insecure" toml:"insecure" json:"insecure"`
	KeepAlive       bool          `mapstructure:"keepalive" toml:"keepalive" json:"keepalive"`
	KeepAlivePeriod time.Duration `mapstructure:"keepalive_period" toml:"keepalive_period" json:"keepalive_period"`
	ConnTimeout     time.Duration `mapstructure:"connection_timeout" toml:"connection_timeout" json:"connection_timeout"`
	FlushPeriod     time.Duration `mapstructure:"flush_period" toml:"flush_period" json:"flush_period"`

	LineFraming    bool  `mapstructure:"line_framing" toml:"line_framing" json:"line_framing"`
	FrameDelimiter uint8 `mapstructure:"delimiter" toml:"delimiter" json:"delimiter"`
}

type HTTPServerDestConfig struct {
	HTTPServerBaseConfig `mapstructure:",squash"`

	TlsBaseConfig         `mapstructure:",squash"`
	ClientAuthType string `mapstructure:"client_auth_type" toml:"client_auth_type" json:"client_auth_type"`

	Port int `mapstructure:"port" toml:"port" json:"port"`

	// format can be empty if the format should be inferred by the accepted mimetypes sent by the client
	Format         string `mapstructure:"format" toml:"format" json:"format"`
	LineFraming    bool   `mapstructure:"line_framing" toml:"line_framing" json:"line_framing"`
	FrameDelimiter uint8  `mapstructure:"delimiter" toml:"delimiter" json:"delimiter"`
	NMessages      int32  `mapstructure:"messages_number" toml:"messages_number" json:"messages_number"`
}

type WebsocketServerDestConfig struct {
	BindAddr    string `mapstructure:"bind_addr" toml:"bind_addr" json:"bind_addr"`
	Port        int    `mapstructure:"port" toml:"port" json:"port"`
	Format      string `mapstructure:"format" toml:"format" json:"format"`
	LogEndPoint string `mapstructure:"log_endpoint" toml:"log_endpoint" json:"log_endpoint"`
	WebEndPoint string `mapstructure:"web_endpoint" toml:"web_endpoint" json:"web_endpoint"`
}

type ElasticDestConfig struct {
	TlsBaseConfig                     `mapstructure:",squash"`
	Insecure            bool          `mapstructure:"insecure" toml:"insecure" json:"insecure"`
	ProxyURL            string        `mapstructure:"proxy_url" toml:"proxy_url" json:"proxy_url"`
	ConnTimeout         time.Duration `mapstructure:"connection_timeout" toml:"connection_timeout" json:"connection_timeout"`
	ConnKeepAlive       bool          `mapstructure:"conn_keepalive" toml:"conn_keepalive" json:"conn_keepalive"`
	ConnKeepAlivePeriod time.Duration `mapstructure:"conn_keepalive_period" toml:"conn_keepalive_period" json:"conn_keepalive_period"`
	Rebind              time.Duration `mapstructure:"rebind" toml:"rebind" json:"rebind"`
	Format              string        `mapstructure:"format" toml:"format" json:"format"`

	URLs                      []string      `mapstructure:"urls" toml:"urls" json:"urls"`
	Username                  string        `mapstructure:"username" toml:"username" json:"username"`
	Password                  string        `mapstructure:"password" toml:"password" json:"password"`
	HealthCheckInterval       time.Duration `mapstructure:"health_check_interval" toml:"health_check_interval" json:"health_check_interval"`
	HealthCheck               bool          `mapstructure:"health_check" toml:"health_check" toml:"health_check" json:"health_check"`
	HealthCheckTimeout        time.Duration `mapstructure:"health_check_timeout" toml:"health_check_timeout" json:"health_check_timeout"`
	HealthCheckTimeoutStartup time.Duration `mapstructure:"health_check_timeout_startup" toml:"health_check_timeout_startup" json:"health_check_timeout_startup"`
	Sniffing                  bool          `mapstructure:"sniffing" toml:"sniffing" json:"sniffing"`
	IndexNameTpl              string        `mapstructure:"index_name_template" toml:"index_name_template" json:"index_name_template"`
	MessagesType              string        `mapstructure:"messages_type" toml:"messages_type" json:"messages_type"`
	BatchSize                 int           `mapstructure:"batch_size" toml:"batch_size" json:"batch_size"`
	FlushPeriod               time.Duration `mapstructure:"flush_period" toml:"flush_period" json:"flush_period"`
	CreateIndices             bool          `mapstructure:"create_indices" toml:"create_indices" json:"create_indices"`

	RefreshInterval time.Duration `mapstructure:"refresh_interval" toml:"refresh_interval" json:"refresh_interval"`
	CheckStartup    bool          `mapstructure:"check_startup" toml:"check_startup" json:"check_startup"`
	NShards         uint          `mapstructure:"shards" toml:"shards" json:"shards"`
	NReplicas       uint          `mapstructure:"replicas" toml:"replicas" json:"replicas"`
}

type RedisDestConfig struct {
	TlsBaseConfig              `mapstructure:",squash"`
	Insecure     bool          `mapstructure:"insecure" toml:"insecure" json:"insecure"`
	Host         string        `mapstructure:"host" toml:"host" json:"host"`
	Port         int           `mapstructure:"port" toml:"port" json:"port"`
	Password     string        `mapstructure:"password" toml:"password" json:"password"`
	Rebind       time.Duration `mapstructure:"rebind" toml:"rebind" json:"rebind"`
	Format       string        `mapstructure:"format" toml:"format" json:"format"`
	Database     int           `mapstructure:"database" toml:"database" json:"database"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout" toml:"dial_timeout" json:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout" toml:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout" toml:"write_timeout" json:"write_timeout"`
}

type HTTPDestConfig struct {
	TlsBaseConfig                     `mapstructure:",squash"`
	Insecure            bool          `mapstructure:"insecure" toml:"insecure" json:"insecure"`
	URL                 string        `mapstructure:"url" toml:"url" json:"url"`
	Method              string        `mapstructure:"method" toml:"method" json:"method"`
	ProxyURL            string        `mapstructure:"proxy_url" toml:"proxy_url" json:"proxy_url"`
	Rebind              time.Duration `mapstructure:"rebind" toml:"rebind" json:"rebind"`
	Format              string        `mapstructure:"format" toml:"format" json:"format"`
	MaxIdleConnsPerHost int           `mapstructure:"max_idle_conns_per_host" toml:"max_idle_conns_per_host" json:"max_idle_conns_per_host"`
	IdleConnTimeout     time.Duration `mapstructure:"idle_conn_timeout" toml:"idle_conn_timeout" json:"idle_conn_timeout"`
	ConnTimeout         time.Duration `mapstructure:"connection_timeout" toml:"connection_timeout" json:"connection_timeout"`
	ConnKeepAlive       bool          `mapstructure:"conn_keepalive" toml:"conn_keepalive" json:"conn_keepalive"`
	ConnKeepAlivePeriod time.Duration `mapstructure:"conn_keepalive_period" toml:"conn_keepalive_period" json:"conn_keepalive_period"`
	BasicAuth           bool          `mapstructure:"basic_auth" toml:"basic_auth" json:"basic_auth"`
	Username            string        `mapstructure:"username" toml:"username" json:"username"`
	Password            string        `mapstructure:"password" toml:"password" json:"password"`
	UserAgent           string        `mapstructure:"user_agent" toml:"user_agent" json:"user_agent"`
	ContentType         string        `mapstructure:"content_type" toml:"content_type" json:"content_type"`
}

type NATSDestConfig struct {
	TlsBaseConfig                  `mapstructure:",squash"`
	Insecure         bool          `mapstructure:"insecure" toml:"insecure" json:"insecure"`
	NServers         []string      `mapstructure:"servers" toml:"servers" json:"servers"`
	Format           string        `mapstructure:"format" toml:"format" json:"format"`
	Name             string        `mapstructure:"name" toml:"name" json:"name"`
	MaxReconnect     int           `mapstructure:"max_reconnect" toml:"max_reconnect" json:"max_reconnect"`
	ReconnectWait    time.Duration `mapstructure:"reconnect_wait" toml:"reconnect_wait" json:"reconnect_wait"`
	ConnTimeout      time.Duration `mapstructure:"connection_timeout" toml:"connection_timeout" json:"connection_timeout"`
	FlusherTimeout   time.Duration `mapstructure:"flusher_timeout" toml:"flusher_timeout" json:"flusher_timeout"`
	PingInterval     time.Duration `mapstructure:"ping_interval" toml:"ping_interval" json:"ping_interval"`
	MaxPingsOut      int           `mapstructure:"max_pings_out" toml:"max_pings_out" json:"max_pings_out"`
	ReconnectBufSize int           `mapstructure:"reconnect_buf_size" toml:"reconnect_buf_size" json:"reconnect_buf_size"`
	Username         string        `mapstructure:"username" toml:"username" json:"username"`
	Password         string        `mapstructure:"password" toml:"password" json:"password"`
	NoRandomize      bool          `mapstructure:"no_randomize" toml:"no_randomize" json:"no_randomize"`
	AllowReconnect   bool          `mapstructure:"allow_reconnect" toml:"allow_reconnect" json:"allow_reconnect"`
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
	FilterSubConfig      `mapstructure:",squash"`
	ConfID  utils.MyULID `mapstructure:"-" toml:"-" json:"conf_id"`
	Enabled bool         `mapstructure:"enabled" toml:"enabled" json:"enabled"`
}

func (c *JournaldConfig) FilterConf() *FilterSubConfig {
	return &c.FilterSubConfig
}

func (c *JournaldConfig) ListenersConf() *ListenersConfig {
	return nil
}

func (c *JournaldConfig) DefaultPort() int {
	return 0
}

type AccountingSourceConfig struct {
	FilterSubConfig       `mapstructure:",squash"`
	ConfID  utils.MyULID  `mapstructure:"-" toml:"-" json:"conf_id"`
	Period  time.Duration `mapstructure:"period" toml:"period" json:"period"`
	Path    string        `mapstructure:"path" toml:"path" json:"path"`
	Enabled bool          `mapstructure:"enabled" toml:"enabled" json:"enabled"`
}

func (c *AccountingSourceConfig) FilterConf() *FilterSubConfig {
	return &c.FilterSubConfig
}

func (c *AccountingSourceConfig) ListenersConf() *ListenersConfig {
	return nil
}

func (c *AccountingSourceConfig) DefaultPort() int {
	return 0
}

type MacOSSourceConfig struct {
	FilterSubConfig        `mapstructure:",squash"`
	ConfID    utils.MyULID `mapstructure:"-" toml:"-" json:"conf_id"`
	Enabled   bool         `mapstructure:"enabled" toml:"enabled" json:"enabled"`
	Level     string       `mapstructure:"level" toml:"level" json:"level"`
	Process   string       `mapstructure:"process" toml:"process" json:"process"`
	Predicate string       `mapstructure:"predicate" toml:"predicate" json:"predicate"`
	Command   string       `mapstructure:"command" toml:"command" json:"command"`
}

func (c *MacOSSourceConfig) FilterConf() *FilterSubConfig {
	return &c.FilterSubConfig
}

func (c *MacOSSourceConfig) ListenersConf() *ListenersConfig {
	return nil
}

func (c *MacOSSourceConfig) DefaultPort() int {
	return 0
}

type FilesystemSourceConfig struct {
	FilterSubConfig            `mapstructure:",squash"`
	BaseDirectory string       `mapstructure:"base_directory" toml:"base_directory" json:"base_directory"`
	Glob          string       `mapstructure:"glob" toml:"glob" json:"glob"`
	Format        string       `mapstructure:"format" toml:"format" json:"format"`
	Encoding      string       `mapstructure:"encoding" toml:"encoding" json:"encoding"`
	ConfID        utils.MyULID `mapstructure:"-" toml:"-" json:"conf_id"`
}

type HTTPServerSourceConfig struct {
	HTTPServerBaseConfig `mapstructure:",squash"`

	FilterSubConfig     `mapstructure:",squash"`
	ConfID utils.MyULID `mapstructure:"-" toml:"-" json:"conf_id"`

	TlsBaseConfig         `mapstructure:",squash"`
	ClientAuthType string `mapstructure:"client_auth_type" toml:"client_auth_type" json:"client_auth_type"`

	Port     int    `mapstructure:"port" toml:"port" json:"port"`
	Format   string `mapstructure:"format" toml:"format" json:"format"`
	Encoding string `mapstructure:"encoding" toml:"encoding" json:"encoding"`
	// should the server accept multiple messages per request
	DisableMultiple bool   `mapstructure:"disable_multiple" toml:"disable_multiple" json:"disable_multiple"`
	FrameDelimiter  string `mapstructure:"delimiter" toml:"delimiter" json:"delimiter"`
	MaxBodySize     int64  `mapstructure:"max_body_size" toml:"max_body_size" json:"max_body_size"`
	MaxMessages     int    `mapstructure:"max_messages" toml:"max_messages" json:"max_messages"`
}

func (c *HTTPServerSourceConfig) FilterConf() *FilterSubConfig {
	return &c.FilterSubConfig
}

func (c *HTTPServerSourceConfig) ListenersConf() *ListenersConfig {
	return nil
}

func (c *HTTPServerSourceConfig) DefaultPort() int {
	return 8081
}

type TCPSourceConfig struct {
	ListenersConfig             `mapstructure:",squash"`
	FilterSubConfig             `mapstructure:",squash"`
	TlsBaseConfig               `mapstructure:",squash"`
	ClientAuthType string       `mapstructure:"client_auth_type" toml:"client_auth_type" json:"client_auth_type"`
	LineFraming    bool         `mapstructure:"line_framing" toml:"line_framing" json:"line_framing"`
	FrameDelimiter string       `mapstructure:"delimiter" toml:"delimiter" json:"delimiter"`
	ConfID         utils.MyULID `mapstructure:"-" toml:"-" json:"conf_id"`
}

func (c *TCPSourceConfig) FilterConf() *FilterSubConfig {
	return &c.FilterSubConfig
}

func (c *TCPSourceConfig) ListenersConf() *ListenersConfig {
	return &c.ListenersConfig
}

func (c *TCPSourceConfig) DefaultPort() int {
	return 1514
}

func (c *FilesystemSourceConfig) FilterConf() *FilterSubConfig {
	return &c.FilterSubConfig
}

func (c *FilesystemSourceConfig) ListenersConf() *ListenersConfig {
	return nil
}

func (c *FilesystemSourceConfig) DefaultPort() int {
	return 0
}

type UDPSourceConfig struct {
	ListenersConfig     `mapstructure:",squash"`
	FilterSubConfig     `mapstructure:",squash"`
	ConfID utils.MyULID `mapstructure:"-" toml:"-" json:"conf_id"`
}

func (c *UDPSourceConfig) FilterConf() *FilterSubConfig {
	return &c.FilterSubConfig
}

func (c *UDPSourceConfig) ListenersConf() *ListenersConfig {
	return &c.ListenersConfig
}

func (c *UDPSourceConfig) DefaultPort() int {
	return 1514
}

type GraylogSourceConfig struct {
	ListenersConfig     `mapstructure:",squash"`
	FilterSubConfig     `mapstructure:",squash"`
	ConfID utils.MyULID `mapstructure:"-" toml:"-" json:"conf_id"`
}

func (c *GraylogSourceConfig) FilterConf() *FilterSubConfig {
	return &c.FilterSubConfig
}

func (c *GraylogSourceConfig) ListenersConf() *ListenersConfig {
	return &c.ListenersConfig
}

func (c *GraylogSourceConfig) DefaultPort() int {
	return 12201
}

type RELPSourceConfig struct {
	ListenersConfig             `mapstructure:",squash"`
	FilterSubConfig             `mapstructure:",squash"`
	TlsBaseConfig               `mapstructure:",squash"`
	ClientAuthType string       `mapstructure:"client_auth_type" toml:"client_auth_type" json:"client_auth_type"`
	LineFraming    bool         `mapstructure:"line_framing" toml:"line_framing" json:"line_framing"`
	FrameDelimiter string       `mapstructure:"delimiter" toml:"delimiter" json:"delimiter"`
	ConfID         utils.MyULID `mapstructure:"-" toml:"-" json:"conf_id"`
}

func (c *RELPSourceConfig) FilterConf() *FilterSubConfig {
	return &c.FilterSubConfig
}

func (c *RELPSourceConfig) ListenersConf() *ListenersConfig {
	return &c.ListenersConfig
}

func (c *RELPSourceConfig) DefaultPort() int {
	return 2514
}

type DirectRELPSourceConfig struct {
	ListenersConfig             `mapstructure:",squash"`
	FilterSubConfig             `mapstructure:",squash"`
	TlsBaseConfig               `mapstructure:",squash"`
	ClientAuthType string       `mapstructure:"client_auth_type" toml:"client_auth_type" json:"client_auth_type"`
	LineFraming    bool         `mapstructure:"line_framing" toml:"line_framing" json:"line_framing"`
	FrameDelimiter string       `mapstructure:"delimiter" toml:"delimiter" json:"delimiter"`
	ConfID         utils.MyULID `mapstructure:"-" toml:"-" json:"conf_id"`
}

func (c *DirectRELPSourceConfig) FilterConf() *FilterSubConfig {
	return &c.FilterSubConfig
}

func (c *DirectRELPSourceConfig) ListenersConf() *ListenersConfig {
	return &c.ListenersConfig
}

func (c *DirectRELPSourceConfig) DefaultPort() int {
	return 3514
}

type Source interface {
	FilterConf() *FilterSubConfig
	ListenersConf() *ListenersConfig
	DefaultPort() int
	SetConfID()
}

type ListenersConfig struct {
	Ports           []int         `mapstructure:"ports" toml:"ports" json:"ports"`
	BindAddr        string        `mapstructure:"bind_addr" toml:"bind_addr" json:"bind_addr"`
	UnixSocketPath  string        `mapstructure:"unix_socket_path" toml:"unix_socket_path" json:"unix_socket_path"`
	Format          string        `mapstructure:"format" toml:"format" json:"format"`
	KeepAlive       bool          `mapstructure:"keepalive" toml:"keepalive" json:"keepalive"`
	KeepAlivePeriod time.Duration `mapstructure:"keepalive_period" toml:"keepalive_period" json:"keepalive_period"`
	Timeout         time.Duration `mapstructure:"timeout" toml:"timeout" json:"timeout"`
	Encoding        string        `mapstructure:"encoding" toml:"encoding" json:"encoding"`
}

type KafkaSourceConfig struct {
	KafkaBaseConfig                 `mapstructure:",squash"`
	KafkaConsumerBaseConfig         `mapstructure:",squash"`
	FilterSubConfig                 `mapstructure:",squash"`
	TlsBaseConfig                   `mapstructure:",squash"`
	Insecure          bool          `mapstructure:"insecure" toml:"insecure" json:"insecure"`
	Format            string        `mapstructure:"format" toml:"format" json:"format"`
	Encoding          string        `mapstructure:"encoding" toml:"encoding" json:"encoding"`
	ConfID            utils.MyULID  `mapstructure:"-" toml:"-" json:"conf_id"`
	SessionTimeout    time.Duration `mapstructure:"session_timeout" toml:"session_timeout" json:"session_timeout"`
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval" toml:"heartbeat_interval" json:"heartbeat_interval"`
	OffsetsMaxRetry   int           `mapstructure:"offsets_max_retry" toml:"offsets_max_retry" json:"offsets_max_retry"`
	GroupID           string        `mapstructure:"group_ip" toml:"group_id" json:"group_id"`
	Topics            []string      `mapstructure:"topics" toml:"topics" json:"topics"`
}

func (c *KafkaSourceConfig) FilterConf() *FilterSubConfig {
	return &c.FilterSubConfig
}

func (c *KafkaSourceConfig) ListenersConf() *ListenersConfig {
	return nil
}

func (c *KafkaSourceConfig) DefaultPort() int {
	return 0
}

type TlsBaseConfig struct {
	TLSEnabled bool   `mapstructure:"tls_enabled" toml:"tls_enabled" json:"tls_enabled"`
	CAFile     string `mapstructure:"ca_file" toml:"ca_file" json:"ca_file"`
	CAPath     string `mapstructure:"ca_path" toml:"ca_path" json:"ca_path"`
	KeyFile    string `mapstructure:"key_file" toml:"key_file" json:"key_file"`
	CertFile   string `mapstructure:"cert_file" toml:"cert_file" json:"cert_file"`
}

type HTTPServerBaseConfig struct {
	BindAddr             string        `mapstructure:"bind_addr" toml:"bind_addr" json:"bind_addr"`
	ReadTimeout          time.Duration `mapstructure:"read_timeout" toml:"read_timeout" json:"read_timeout"`
	ReadHeaderTimeout    time.Duration `mapstructure:"read_header_timeout" toml:"read_header_timeout" json:"read_header_timeout"`
	WriteTimeout         time.Duration `mapstructure:"write_timeout" toml:"write_timeout" json:"write_timeout"`
	IdleTimeout          time.Duration `mapstructure:"idle_timeout" toml:"idle_timeout" json:"idle_timeout"`
	MaxHeaderBytes       int           `mapstructure:"max_header_bytes" toml:"max_header_bytes" json:"max_header_bytes"`
	DisableConnKeepAlive bool          `mapstructure:"disable_conn_keepalive" toml:"disable_conn_keepalive" json:"disable_conn_keepalive"`
	ConnKeepAlivePeriod  time.Duration `mapstructure:"conn_keepalive_period" toml:"conn_keepalive_period" json:"conn_keepalive_period"`
	DisableHTTPKeepAlive bool          `mapstructure:"disable_http_keepalive" toml:"disable_http_keepalive" json:"disable_http_keepalive"`
}

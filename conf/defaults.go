package conf

import (
	"github.com/spf13/viper"
)

func SetDefaults(v *viper.Viper) {
	v.SetDefault("syslog.port", 2514)
	v.SetDefault("syslog.bind_addr", "127.0.0.1")

	
}

/*
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
RequiredAcks             int                     `mapstructure:"required_acks" toml:"required_acks"`
ProducerTimeout          time.Duration           `mapstructure:"producer_timeout" toml:"producer_timeout"`
Compression              string                  `mapstructure:"compression" toml:"compression"`
FlushBytes               int                     `mapstructure:"flush_bytes" toml:"flush_bytes"`
FlushMessages            int                     `mapstructure:"flush_messages" toml:"flush_messages"`
FlushFrequency           time.Duration           `mapstructure:"flush_frequency" toml:"flush_frequency"`
FlushMessagesMax         int                     `mapstructure:"flush_messages_max" toml:"flush_messages_max"`
RetrySendMax             int                     `mapstructure:"retry_send_max" toml:"retry_send_max"`
RetrySendBackoff         time.Duration           `mapstructure:"retry_send_backoff" toml:"retry_send_backoff"`
*/

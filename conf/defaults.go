package conf

import (
	"github.com/spf13/viper"
	sarama "gopkg.in/Shopify/sarama.v1"
)

func SetDefaults(v *viper.Viper) {
	v.SetDefault("kafka.brokers", []string{"kafka1", "kafka2", "kafka3"})
	v.SetDefault("kafka.client_id", "relp2kafka")
	v.SetDefault("kafka.version", "0.10.1.0")
	v.SetDefault("kafka.channel_buffer_size", 256)
	v.SetDefault("kafka.max_open_requests", 5)
	v.SetDefault("kafka.dial_timeout", "30s")
	v.SetDefault("kafka.read_timeout", "30s")
	v.SetDefault("kafka.write_timeout", "30s")
	v.SetDefault("kafka.keepalive", 0)
	v.SetDefault("kafka.metadata_retry_max", 3)
	v.SetDefault("kafka.metadata_retry_backoff", "250ms")
	v.SetDefault("kafka.metadata_refresh_frequency", "10m")
	v.SetDefault("kafka.message_bytes_max", 1000000)
	v.SetDefault("kafka.required_acks", sarama.WaitForAll)
	v.SetDefault("kafka.producer_timeout", "10s")
	v.SetDefault("kafka.compression", "snappy")
	v.SetDefault("kafka.flush_bytes", 0)
	v.SetDefault("kafka.flush_messages", 0)
	v.SetDefault("kafka.flush_frequency", 0)
	v.SetDefault("kafka.flush_messages_max", 0)
	v.SetDefault("kafka.retry_send_max", 3)
	v.SetDefault("kafka.retry_send_backoff", "100ms")

	v.SetDefault("store.dirname", "/var/lib/relp2kafka")
}

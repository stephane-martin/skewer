package conf

import (
	"github.com/spf13/viper"
	sarama "gopkg.in/Shopify/sarama.v1"
)

func SetDefaults(v *viper.Viper) {
	SetKafkaDefaults(v, true)
	SetStoreDefaults(v, true)
	SetJournaldDefaults(v, true)
}

func SetJournaldDefaults(v *viper.Viper, prefixed bool) {
	var prefix string
	if prefixed {
		prefix = "journald."
	}
	v.SetDefault(prefix+"topic_tmpl", "rsyslog-{{.Appname}}")
	v.SetDefault(prefix+"partition_key_tmpl", "mypk-{{.Hostname}}")
}

func SetKafkaDefaults(v *viper.Viper, prefixed bool) {
	var prefix string
	if prefixed {
		prefix = "kafka."
	}
	v.SetDefault(prefix+"brokers", []string{"kafka1", "kafka2", "kafka3"})
	v.SetDefault(prefix+"client_id", "relp2kafka")
	v.SetDefault(prefix+"version", "0.10.1.0")
	v.SetDefault(prefix+"channel_buffer_size", 256)
	v.SetDefault(prefix+"max_open_requests", 5)
	v.SetDefault(prefix+"dial_timeout", "30s")
	v.SetDefault(prefix+"read_timeout", "30s")
	v.SetDefault(prefix+"write_timeout", "30s")
	v.SetDefault(prefix+"keepalive", 0)
	v.SetDefault(prefix+"metadata_retry_max", 3)
	v.SetDefault(prefix+"metadata_retry_backoff", "250ms")
	v.SetDefault(prefix+"metadata_refresh_frequency", "10m")
	v.SetDefault(prefix+"message_bytes_max", 1000000)
	v.SetDefault(prefix+"required_acks", sarama.WaitForAll)
	v.SetDefault(prefix+"producer_timeout", "10s")
	v.SetDefault(prefix+"compression", "snappy")
	v.SetDefault(prefix+"flush_bytes", 0)
	v.SetDefault(prefix+"flush_messages", 0)
	v.SetDefault(prefix+"flush_frequency", 0)
	v.SetDefault(prefix+"flush_messages_max", 0)
	v.SetDefault(prefix+"retry_send_max", 3)
	v.SetDefault(prefix+"retry_send_backoff", "100ms")
}

func SetStoreDefaults(v *viper.Viper, prefixed bool) {
	var prefix string
	if prefixed {
		prefix = "store."
	}
	v.SetDefault(prefix+"dirname", "/var/lib/relp2kafka")
	v.SetDefault(prefix+"max_size", 64<<20)
}

package conf

import (
	"os/exec"

	"github.com/spf13/viper"
	sarama "gopkg.in/Shopify/sarama.v1"
)

func SetDefaults(v *viper.Viper) {
	SetKafkaDefaults(v, true)
	SetStoreDefaults(v, true)
	SetJournaldDefaults(v, true)
	SetAuditDefaults(v, true)
	SetMetricsDefaults(v, true)
}

func SetMetricsDefaults(v *viper.Viper, prefixed bool) {
	prefix := ""
	if prefixed {
		prefix = "metrics."
	}
	v.SetDefault(prefix+"path", "/metrics")
	v.SetDefault(prefix+"port", 8080)
}

func SetAuditDefaults(v *viper.Viper, prefixed bool) {
	prefix := ""
	if prefixed {
		prefix = "audit."
	}
	v.SetDefault(prefix+"enabled", false)
	v.SetDefault(prefix+"socket_buffer", 0)
	v.SetDefault(prefix+"events_min", 1300)
	v.SetDefault(prefix+"events_max", 1399)
	v.SetDefault(prefix+"message_tracking", true)
	v.SetDefault(prefix+"log_out_of_order", false)
	v.SetDefault(prefix+"max_out_of_order", 500)
	v.SetDefault(prefix+"appname", "audit")
	v.SetDefault(prefix+"severity", 6)
	v.SetDefault(prefix+"facility", 0)
	v.SetDefault(prefix+"topic_tmpl", "linux-audit-topic")
	v.SetDefault(prefix+"partition_key_tmpl", "pk-{{.Hostname}}")
	v.SetDefault(prefix+"encoding", "utf8")

}

func SetJournaldDefaults(v *viper.Viper, prefixed bool) {
	var prefix string
	if prefixed {
		prefix = "journald."
	}
	_, err := exec.LookPath("systemctl")
	if err == nil {
		v.SetDefault(prefix+"enabled", true)
	} else {
		v.SetDefault(prefix+"enabled", false)
	}
	v.SetDefault(prefix+"topic_tmpl", "journald-{{.Appname}}")
	v.SetDefault(prefix+"partition_key_tmpl", "pk-{{.Hostname}}")
	v.SetDefault(prefix+"encoding", "utf8")
}

func SetKafkaDefaults(v *viper.Viper, prefixed bool) {
	var prefix string
	if prefixed {
		prefix = "kafka."
	}
	v.SetDefault(prefix+"brokers", []string{"kafka1", "kafka2", "kafka3"})
	v.SetDefault(prefix+"client_id", "skewer")
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
	v.SetDefault(prefix+"dirname", "/var/lib/skewer")
	v.SetDefault(prefix+"max_size", 64<<20)
}

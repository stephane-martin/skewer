package conf

import (
	"compress/flate"
	"os"

	sarama "github.com/Shopify/sarama"
	nats "github.com/nats-io/go-nats"
	"github.com/spf13/viper"
)

type defaultFunc func(v *viper.Viper, prefixed bool)

func SetDefaults(v *viper.Viper) {
	funcs := []defaultFunc{
		SetKafkaDefaults,
		SetStoreDefaults,
		SetJournaldDefaults,
		SetMetricsDefaults,
		SetAccountingDefaults,
		SetMetricsDefaults,
		SetUdpDestDefaults,
		SetTcpDestDefaults,
		SetRelpDestDefaults,
		SetFileDestDefaults,
		SetStderrDestDefaults,
		SetGraylogDestDefaults,
		SetHTTPDestDefaults,
		SetHTTPServerDestDefaults,
		SetWebsocketServerDestDefaults,
		SetNatsDestDefaults,
		SetMainDefaults,
	}
	for _, f := range funcs {
		f(v, true)
	}
}

func SetNatsDestDefaults(v *viper.Viper, prefixed bool) {
	prefix := ""
	if prefixed {
		prefix = "nats_destination."
	}
	v.SetDefault(prefix+"format", "fulljson")
	v.SetDefault(prefix+"name", "skewer")
	v.SetDefault(prefix+"max_reconnect", nats.DefaultMaxReconnect)
	v.SetDefault(prefix+"reconnect_wait", nats.DefaultReconnectWait)
	v.SetDefault(prefix+"connection_timeout", nats.DefaultTimeout)
	v.SetDefault(prefix+"ping_interval", nats.DefaultPingInterval)
	v.SetDefault(prefix+"max_pings_out", nats.DefaultMaxPingOut)
	v.SetDefault(prefix+"reconnect_buf_size", nats.DefaultReconnectBufSize)
	v.SetDefault(prefix+"allow_reconnect", true)
	v.SetDefault(prefix+"no_randomize", false)
	v.SetDefault(prefix+"flusher_timeout", 0)
}

func SetHTTPServerDestDefaults(v *viper.Viper, prefixed bool) {
	prefix := ""
	if prefixed {
		prefix = "httpserver_destination."
	}
	v.SetDefault(prefix+"format", "fulljson")
	v.SetDefault(prefix+"content_type", "auto")
	v.SetDefault(prefix+"bind_addr", "127.0.0.1")
	v.SetDefault(prefix+"port", "8514")
	v.SetDefault(prefix+"delimiter", 10)
	v.SetDefault(prefix+"line_framing", true)
	v.SetDefault(prefix+"messages_number", 8*1024)
}

func SetWebsocketServerDestDefaults(v *viper.Viper, prefixed bool) {
	prefix := ""
	if prefixed {
		prefix = "websocketserver_destination."
	}
	v.SetDefault(prefix+"format", "fulljson")
	v.SetDefault(prefix+"bind_addr", "127.0.0.1")
	v.SetDefault(prefix+"port", "8515")
	v.SetDefault(prefix+"log_endpoint", "/logs")
	v.SetDefault(prefix+"web_endpoint", "/web")
}

func SetHTTPDestDefaults(v *viper.Viper, prefixed bool) {
	prefix := ""
	if prefixed {
		prefix = "http_destination."
	}
	v.SetDefault(prefix+"format", "fulljson")
	v.SetDefault(prefix+"max_idle_conns_per_host", 2)
	v.SetDefault(prefix+"idle_conn_timeout", "90s")
	v.SetDefault(prefix+"connection_timeout", "10s")
	v.SetDefault(prefix+"conn_keepalive", true)
	v.SetDefault(prefix+"conn_keepalive_period", "30s")
	v.SetDefault(prefix+"user_agent", "skewer/"+Version)
	v.SetDefault(prefix+"method", "POST")
	v.SetDefault(prefix+"content_type", "auto")
}

func SetGraylogDestDefaults(v *viper.Viper, prefixed bool) {
	prefix := ""
	if prefixed {
		prefix = "graylog_destination."
	}
	v.SetDefault(prefix+"host", "127.0.0.1")
	v.SetDefault(prefix+"port", 12201)
	v.SetDefault(prefix+"mode", "udp")
	v.SetDefault(prefix+"max_reconnect", 3)
	v.SetDefault(prefix+"reconnect_delay", "1s")
	v.SetDefault(prefix+"compression_level", flate.BestSpeed)
	v.SetDefault(prefix+"compression_type", "gzip")
}

func SetRelpDestDefaults(v *viper.Viper, prefixed bool) {
	prefix := ""
	if prefixed {
		prefix = "relp_destination."
	}
	v.SetDefault(prefix+"host", "127.0.0.1")
	v.SetDefault(prefix+"port", 1515)
	v.SetDefault(prefix+"format", "rfc5424")
	v.SetDefault(prefix+"keepalive", true)
	v.SetDefault(prefix+"keepalive_period", "75s")
	v.SetDefault(prefix+"window_size", 128)
	v.SetDefault(prefix+"connection_timeout", "10s")
	v.SetDefault(prefix+"relp_timeout", "90s")
	v.SetDefault(prefix+"flush_period", "1s")
}

func SetFileDestDefaults(v *viper.Viper, prefixed bool) {
	prefix := ""
	if prefixed {
		prefix = "file_destination."
	}
	v.SetDefault(prefix+"filename", "/var/log/skewer/{{.Date}}/{{.Appname}}.log")
	v.SetDefault(prefix+"sync", false)
	v.SetDefault(prefix+"sync_period", "5s")
	v.SetDefault(prefix+"flush_period", "1s")
	v.SetDefault(prefix+"buffer_size", 16384)
	v.SetDefault(prefix+"open_files_cache", 128)
	v.SetDefault(prefix+"open_file_timeout", "1m")
	v.SetDefault(prefix+"gzip", false)
	v.SetDefault(prefix+"gzip_level", 5)
	v.SetDefault(prefix+"format", "file")
}

func SetStderrDestDefaults(v *viper.Viper, prefixed bool) {
	prefix := ""
	if prefixed {
		prefix = "stderr_destination."
	}
	v.SetDefault(prefix+"format", "fulljson")
}

func SetUdpDestDefaults(v *viper.Viper, prefixed bool) {
	prefix := ""
	if prefixed {
		prefix = "udp_destination."
	}
	v.SetDefault(prefix+"host", "127.0.0.1")
	v.SetDefault(prefix+"port", 1514)
	v.SetDefault(prefix+"format", "rfc5424")
}

func SetTcpDestDefaults(v *viper.Viper, prefixed bool) {
	prefix := ""
	if prefixed {
		prefix = "tcp_destination."
	}
	v.SetDefault(prefix+"host", "127.0.0.1")
	v.SetDefault(prefix+"port", 1514)
	v.SetDefault(prefix+"format", "rfc5424")
	v.SetDefault(prefix+"delimiter", 10)
	v.SetDefault(prefix+"keepalive", true)
	v.SetDefault(prefix+"keepalive_period", "75s")
	v.SetDefault(prefix+"connection_timeout", "10s")
	v.SetDefault(prefix+"flush_period", "1s")
}

func SetMainDefaults(v *viper.Viper, prefixed bool) {
	prefix := ""
	if prefixed {
		prefix = "main."
	}
	v.SetDefault(prefix+"direct_relp", false)
	v.SetDefault(prefix+"max_input_message_size", 65536)
	v.SetDefault(prefix+"input_queue_size", 1024)
	v.SetDefault(prefix+"destination", "stderr")
	v.SetDefault(prefix+"encrypt_ipc", true)
}

func SetAccountingDefaults(v *viper.Viper, prefixed bool) {
	prefix := ""
	if prefixed {
		prefix = "accounting."
	}
	v.SetDefault(prefix+"enabled", false)
	v.SetDefault(prefix+"path", AccountingPath)
	v.SetDefault(prefix+"period", "1s")
}

func SetMetricsDefaults(v *viper.Viper, prefixed bool) {
	prefix := ""
	if prefixed {
		prefix = "metrics."
	}
	v.SetDefault(prefix+"path", "/metrics")
	v.SetDefault(prefix+"port", 8080)
}

func SetJournaldDefaults(v *viper.Viper, prefixed bool) {
	var prefix string
	if prefixed {
		prefix = "journald."
	}
	v.SetDefault(prefix+"enabled", os.Getenv("SKEWER_HAVE_SYSTEMCTL") == "TRUE")
	v.SetDefault(prefix+"encoding", "utf8")
}

func SetKafkaDefaults(v *viper.Viper, prefixed bool) {
	var prefix string
	if prefixed {
		prefix = "kafka_destination."
	}
	// common parameters
	v.SetDefault(prefix+"brokers", []string{"kafka1", "kafka2", "kafka3"})
	v.SetDefault(prefix+"client_id", "skewerd")
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

	// producer parameters
	v.SetDefault(prefix+"message_bytes_max", 1000000)
	v.SetDefault(prefix+"required_acks", sarama.WaitForAll)
	v.SetDefault(prefix+"flush_bytes", 0)
	v.SetDefault(prefix+"flush_messages", 0)
	v.SetDefault(prefix+"flush_frequency", 0)
	v.SetDefault(prefix+"flush_messages_max", 0)
	v.SetDefault(prefix+"retry_send_max", 3)
	v.SetDefault(prefix+"retry_send_backoff", "100ms")
	v.SetDefault(prefix+"producer_timeout", "10s")
	v.SetDefault(prefix+"compression", "snappy")
	v.SetDefault(prefix+"partitioner", "hash")

	v.SetDefault(prefix+"format", "fulljson")
}

func SetStoreDefaults(v *viper.Viper, prefixed bool) {
	var prefix string
	if prefixed {
		prefix = "store."
	}
	v.SetDefault(prefix+"dirname", "/var/lib/skewer")
	v.SetDefault(prefix+"max_table_size", 64<<20)
	v.SetDefault(prefix+"value_log_file_size", 64<<20)
	v.SetDefault(prefix+"batch_size", 5000)
	v.SetDefault(prefix+"add_missing_msgid", true)
}

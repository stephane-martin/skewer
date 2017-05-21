package conf

import (
	"github.com/spf13/viper"
)

func SetDefaults(v *viper.Viper) {
	v.SetDefault("syslog.port", 1514)
	v.SetDefault("syslog.bind_addr", "127.0.0.1")
}

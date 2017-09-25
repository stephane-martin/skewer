package cmd

import (
	"github.com/spf13/cobra"
)

var configDirName string
var storeDirname string
var consulDC string
var consulToken string
var consulAddr string
var consulPrefix string
var consulCAFile string
var consulCAPath string
var consulCertFile string
var consulKeyFile string
var consulInsecure bool

var RootCmd = &cobra.Command{
	Use:   "skewer",
	Short: "skewer is a Syslog server that forwards message to Kafka",
	Long: `skewer is a Syslog server. It implements listening on TCP, UDP, and
RELP. It can also retrieve messages from Journald on Linux. Syslog messages
are forwarded to Kafka. Skewer configuration can be pulled from Consul. Skewer
supports TLS for the Syslog services, for Consul connection and for Kafka
connection.`,
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return RootCmd.Execute()
}

func init() {
	RootCmd.PersistentFlags().StringVar(&configDirName, "config", "/etc", "configuration directory")
	RootCmd.PersistentFlags().StringVar(&storeDirname, "store", "/var/lib/skewer", "store directory")
	RootCmd.PersistentFlags().StringVar(&consulAddr, "consul-addr", "", "Consul address (ex: http://127.0.0.1:8500)")
	RootCmd.PersistentFlags().StringVar(&consulDC, "consul-dc", "", "Consul datacenter")
	RootCmd.PersistentFlags().StringVar(&consulToken, "consul-token", "", "Consul token")
	RootCmd.PersistentFlags().StringVar(&consulPrefix, "consul-prefix", "skewer", "Where to find configuration in Consul KV")
	RootCmd.PersistentFlags().StringVar(&consulCAFile, "consul-ca-file", "", "optional path to the CA certificate used for Consul")
	RootCmd.PersistentFlags().StringVar(&consulCAPath, "consul-ca-path", "", "optional path to a directory of CA certificates to use for Consul")
	RootCmd.PersistentFlags().StringVar(&consulCertFile, "consul-cert-file", "", "optional path to the client certificate for Consul")
	RootCmd.PersistentFlags().StringVar(&consulKeyFile, "consul-key-file", "", "optional path to the client private key for Consul")
	RootCmd.PersistentFlags().BoolVar(&consulInsecure, "consul-insecure", false, "if set to true will disable TLS host verification")
}

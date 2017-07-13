package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/inconshreveable/log15"
	"github.com/spf13/cobra"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/consul"
)

// printConfigCmd represents the printConfig command
var printConfigCmd = &cobra.Command{
	Use:   "print-config",
	Short: "Print skewer configuration as a TOML export",
	Long: `With print-config you can have a TOML view of the current skewer
configuration. The exported configuration will take Consul configuration in
account, if you provide the necessary Consul flags on the command line.`,
	Run: func(cmd *cobra.Command, args []string) {

		params := consul.ConnParams{
			Address:    consulAddr,
			Datacenter: consulDC,
			Token:      consulToken,
			CAFile:     consulCAFile,
			CAPath:     consulCAPath,
			CertFile:   consulCertFile,
			KeyFile:    consulKeyFile,
			Insecure:   consulInsecure,
		}

		c, _, err := conf.InitLoad(context.Background(), configDirName, params, consulPrefix, log15.New())
		if err != nil {
			fmt.Printf("Error happened: %s\n", err)
			os.Exit(-1)
		}
		fmt.Println(c)
	},
}

func init() {
	RootCmd.AddCommand(printConfigCmd)
}

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
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
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

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// printConfigCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// printConfigCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

package cmd

import (
	"context"
	"fmt"

	"github.com/inconshreveable/log15"
	"github.com/spf13/cobra"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/consul"
	"github.com/stephane-martin/skewer/store"
)

// printStoreCmd represents the printStore command
var printStoreCmd = &cobra.Command{
	Use:   "print-store",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("print-store called")

		var err error
		var c *conf.GConfig
		var st store.Store
		ctx, cancel := context.WithCancel(context.Background())
		logger := log15.New()

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

		c, _, err = conf.InitLoad(ctx, configDirName, params, consulPrefix, logger)
		if err != nil {
			fmt.Println("bleh")
			return
		}

		// prepare the message store
		st, err = store.NewStore(ctx, c.Store, logger)
		if err != nil {
			fmt.Println("Can't create the message Store", "error", err)
			return
		}
		defer func() {
			close(st.Ack())
			close(st.Nack())
			close(st.ProcessingErrors())
			cancel()
			st.WaitFinished()
		}()

		readyMap, failedMap, sentMap := st.ReadAllBadgers()

		fmt.Println("Ready")
		for k, v := range readyMap {
			fmt.Printf("%s %s\n", k, v)
		}
		fmt.Println()

		fmt.Println("Failed")
		for k, v := range failedMap {
			fmt.Printf("%s %s\n", k, v)
		}
		fmt.Println()

		fmt.Println("Sent")
		for k, v := range sentMap {
			fmt.Printf("%s %s\n", k, v)
		}
		fmt.Println()

	},
}

func init() {
	RootCmd.AddCommand(printStoreCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// printStoreCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// printStoreCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

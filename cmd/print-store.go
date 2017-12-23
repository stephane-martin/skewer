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
	Short: "Debugging stats about the Store",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("print-store called")

		var err error
		var c conf.BaseConfig
		var st store.Store
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
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
			Prefix:     consulPrefix,
		}

		c, _, err = conf.InitLoad(ctx, configDirName, params, nil, logger)
		if err != nil {
			fmt.Println("bleh", err)
			return
		}
		c.Store.Dirname = storeDirname

		// prepare the message store
		st, err = store.NewStore(ctx, c.Store, nil, conf.Stderr, false, logger)
		if err != nil {
			fmt.Println("Can't create the message Store", "path", c.Store.Dirname, "error", err)
			return
		}
		//defer st.WaitFinished()

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
	},
}

func init() {
	RootCmd.AddCommand(printStoreCmd)
}

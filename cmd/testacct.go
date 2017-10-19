package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/stephane-martin/skewer/accounting"
	"github.com/stephane-martin/skewer/conf"
)

var testacctCmd = &cobra.Command{
	Use:   "testacct",
	Short: "Follow the acct file and print updates",
	Run: func(cmd *cobra.Command, args []string) {
		f, err := os.Open(conf.AccountingPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		defer f.Close()
		buf := make([]byte, accounting.Ssize)
		var acct accounting.Acct
		tick := accounting.Tick()
		for {
			_, err := io.ReadAtLeast(f, buf, accounting.Ssize)
			if err != nil {
				fmt.Println("pausing")
				time.Sleep(5 * time.Second)
				continue
			}
			acct = accounting.MakeAcct(buf, tick)
			fmt.Println(acct.Properties())
		}
	},
}

func init() {
	RootCmd.AddCommand(testacctCmd)
}

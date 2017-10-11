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

// acctCmd represents the acct command
var acctCmd = &cobra.Command{
	Use:   "acct",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
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
	RootCmd.AddCommand(acctCmd)
}

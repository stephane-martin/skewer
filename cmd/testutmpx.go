package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stephane-martin/skewer/sys/utmpx"
)

// testutmpxCmd represents the testutmpx command
var testutmpxCmd = &cobra.Command{
	Use:   "testutmpx",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		b, _ := json.MarshalIndent(utmpx.All(), "", "  ")
		fmt.Fprintln(os.Stderr, string(b))
	},
}

func init() {
	RootCmd.AddCommand(testutmpxCmd)
}

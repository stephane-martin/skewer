package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stephane-martin/skewer/conf"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print skewer version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Version: %s, Commit: %s\n", conf.Version, conf.GitCommit)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}

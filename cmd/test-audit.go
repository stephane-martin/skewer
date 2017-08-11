// +build linux

package cmd

import (
	"context"
	"fmt"

	"github.com/inconshreveable/log15"
	"github.com/spf13/cobra"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/services"
	"github.com/stephane-martin/skewer/utils"
)

// test-auditCmd represents the test-audit command
var testauditCmd = &cobra.Command{
	Use:   "test-audit",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("test-audit called")

		c := conf.AuditConfig{
			Appname:       "app",
			Enabled:       true,
			EventsMax:     1399,
			EventsMin:     1300,
			LogOutOfOrder: true,
			MaxOutOfOrder: 500,
			SocketBuffer:  0,
			Facility:      0,
			Severity:      6,
		}
		logger := log15.New()
		ctx, _ := context.WithCancel(context.Background())
		generator := utils.Generator(ctx, logger)
		auditsvc := services.NewAuditService(nil, generator, nil, logger)
		err := auditsvc.Start(ctx, &c)
		if err != nil {
			fmt.Println("doh", err)
		} else {
			auditsvc.WaitFinished()
		}
	},
}

func init() {
	RootCmd.AddCommand(testauditCmd)
}

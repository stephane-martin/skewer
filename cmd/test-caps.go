// +build linux

package cmd

import (
	"fmt"
	"os"

	"github.com/hashicorp/go-version"
	"github.com/shirou/gopsutil/host"
	"github.com/spf13/cobra"
	"github.com/stephane-martin/skewer/sys/capabilities"
	"github.com/syndtr/gocapability/capability"
)

// test-capsCmd represents the test-caps command
var testcapsCmd = &cobra.Command{
	Use:   "test-caps",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("test-caps called")
		caps, err := capability.NewPid(os.Getpid())
		if err == nil {
			fmt.Println(caps)
			fmt.Println("effective:", caps.StringCap(capability.EFFECTIVE))
			fmt.Println("ambient:", caps.StringCap(capability.AMBIENT))
			fmt.Println("setpcap", caps.Get(capability.EFFECTIVE, capability.CAP_SETPCAP))
			fmt.Println("setuid", caps.Get(capability.EFFECTIVE, capability.CAP_SETUID))
			fmt.Println("setgid", caps.Get(capability.EFFECTIVE, capability.CAP_SETGID))
			fmt.Println("bind", caps.Get(capability.EFFECTIVE, capability.CAP_NET_BIND_SERVICE))
			fmt.Println("audit_read", caps.Get(capability.EFFECTIVE, capability.CAP_AUDIT_READ))
			fmt.Println("lockmem", caps.Get(capability.EFFECTIVE, capability.CAP_IPC_LOCK))

			err = capabilities.Drop(1000, 1000)
			if err != nil {
				fmt.Println("dropping caps failed:", err)
			}

			caps, err := capability.NewPid(os.Getpid())
			if err != nil {
				fmt.Println("load caps error", err)
			}

			fmt.Println(caps)
			fmt.Println("setpcap", caps.Get(capability.EFFECTIVE, capability.CAP_SETPCAP))
			fmt.Println("setuid", caps.Get(capability.EFFECTIVE, capability.CAP_SETUID))
			fmt.Println("setgid", caps.Get(capability.EFFECTIVE, capability.CAP_SETGID))
			fmt.Println("bind", caps.Get(capability.EFFECTIVE, capability.CAP_NET_BIND_SERVICE))

			fmt.Println(os.Getuid(), os.Geteuid())
			v, err := host.KernelVersion()
			if err == nil {
				fmt.Println(v)
			}
			curv := version.Must(version.NewVersion(v))
			minv := version.Must(version.NewVersion("4.3"))
			fmt.Println(curv.Compare(minv))

		} else {
			fmt.Println(err)
		}
	},
}

func init() {
	RootCmd.AddCommand(testcapsCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// test-capsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// test-capsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

}

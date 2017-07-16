// +build linux

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stephane-martin/skewer/sys"
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

			/*
				caps.Clear(capability.CAPS)
				caps.Clear(capability.BOUNDS)
				caps.Clear(capability.AMBS)
				caps.Set(capability.CAPS|capability.BOUNDING, capability.CAP_NET_BIND_SERVICE, capability.CAP_SETUID, capability.CAP_SETGID, capability.CAP_SETPCAP)

				err = caps.Apply(capability.BOUNDING)
				if err != nil {
					fmt.Println("apply caps bounding error", err)
				}

				err = caps.Apply(capability.CAPS)
				if err != nil {
					fmt.Println("apply caps effective error", err)
				}

				err = caps.Apply(capability.AMBIENT)
				if err != nil {
					fmt.Println("apply caps ambient error", err)
				}

				err = sys.KeepCaps()
				if err != nil {
					fmt.Println("keepcaps error", err)
				}

				err = sys.NoNewPriv()
				if err != nil {
					fmt.Println("nonewpriv error", err)
				}

				sys.Setuid(1000)
				sys.Setgid(1000)

				caps.Unset(capability.CAPS|capability.BOUNDING, capability.CAP_SETUID, capability.CAP_SETGID, capability.CAP_SETPCAP)
				caps.Apply(capability.BOUNDING)
				caps.Apply(capability.CAPS)

			*/

			capquery, _ := sys.NewCapabilitiesQuery()
			err = capquery.Drop(1000, 1000)
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

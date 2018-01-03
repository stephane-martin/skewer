// Copyright © 2018 NAME HERE <EMAIL ADDRESS>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	nats "github.com/nats-io/go-nats"
	"github.com/spf13/cobra"
)

// natsdebugCmd represents the natsdebug command
var natsdebugCmd = &cobra.Command{
	Use:   "natsdebug",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("natsdebug called")
		conn, err := nats.Connect(nats.DefaultURL)
		defer conn.Close()
		if err != nil {
			fmt.Println(err)
			return
		}
		conn.Subscribe(">", handler)
		sigchan := make(chan os.Signal, 1)
		signal.Notify(sigchan, syscall.SIGTERM, syscall.SIGINT)
		<-sigchan
	},
}

func handler(msg *nats.Msg) {
	fmt.Println(msg.Subject, string(msg.Data))
}

func init() {
	RootCmd.AddCommand(natsdebugCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// natsdebugCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// natsdebugCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// Copyright © 2017 NAME HERE <EMAIL ADDRESS>
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
	"log"
	"net/http"
	"net/http/httputil"
	"os"

	"github.com/spf13/cobra"
)

// httpdebugCmd represents the httpdebug command
var httpdebugCmd = &cobra.Command{
	Use:   "httpdebug",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("httpdebug called")
		log.Println("starting server, listening on port " + getServerPort())

		http.HandleFunc("/", EchoHandler)
		http.ListenAndServe(":"+getServerPort(), nil)
	},
}

func init() {
	RootCmd.AddCommand(httpdebugCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// httpdebugCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// httpdebugCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// DefaultPort is the default port to use if once is not specified by the SERVER_PORT environment variable
const DefaultPort = "7893"

func getServerPort() string {
	port := os.Getenv("SERVER_PORT")
	if port != "" {
		return port
	}

	return DefaultPort
}

func EchoHandler(writer http.ResponseWriter, request *http.Request) {
	buf, err := httputil.DumpRequest(request, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, "DumpRequest error:", err)
		writer.WriteHeader(500)
		return
	}
	fmt.Println(string(buf))
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.WriteHeader(200)
}

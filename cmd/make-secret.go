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
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// makeSecretCmd represents the makeSecret command
var makeSecretCmd = &cobra.Command{
	Use:   "make-secret",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		secretb := make([]byte, 32)
		_, err := rand.Read(secretb)
		if err != nil {
			fmt.Println("Error happened", err)
			os.Exit(-1)
		} else {
			secret := base64.URLEncoding.EncodeToString(secretb)
			fmt.Println(string(secret))
		}
	},
}

func init() {
	RootCmd.AddCommand(makeSecretCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// makeSecretCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// makeSecretCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

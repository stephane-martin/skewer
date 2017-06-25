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
	"time"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/relp2kafka/javascript"
	"github.com/stephane-martin/relp2kafka/model"

	"github.com/spf13/cobra"
)

// testjsCmd represents the testjs command
var testjsCmd = &cobra.Command{
	Use:   "testjs",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("testjs called")
		logger := log15.New()
		ffunc := `function FilterMessages(m) { m.Message="bla"; return true; }`
		tfunc := `function Topic(m) { return "topic-" + m.Appname; }`
		env := javascript.New(ffunc, tfunc, nil, "", nil, logger)
		m := model.SyslogMessage{}
		m.TimeReported = time.Now()
		m.Appname = "myapp"
		ma := map[string]string{"zog": "zogzog"}
		m.Properties = map[string]interface{}{"foo": "bar", "ma": ma}
		m2 := env.FilterMessage(&m)
		fmt.Println(m2)
		topic := env.Topic(&m)
		fmt.Println(topic)

	},
}

func init() {
	RootCmd.AddCommand(testjsCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// testjsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// testjsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

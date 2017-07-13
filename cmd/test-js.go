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
	"github.com/stephane-martin/skewer/javascript"
	"github.com/stephane-martin/skewer/model"

	"github.com/spf13/cobra"
)

var testjsCmd = &cobra.Command{
	Use:   "testjs",
	Short: "Debugging stuff for the Ecmascript VM",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("testjs called")
		logger := log15.New()
		ffunc := `function FilterMessages(m) { m.Message="bla"; return FILTER.DROPPED; }`
		tfunc := `function Topic(m) { return "topic-" + m.Appname; }`
		env := javascript.NewFilterEnvironment(ffunc, tfunc, "", "", "", logger)
		m := model.SyslogMessage{}
		m.TimeReported = time.Now()
		m.Appname = "myapp"
		ma := map[string]string{"zog": "zogzog"}
		m.Properties = map[string]interface{}{"foo": "bar", "ma": ma}
		m2, result, err := env.FilterMessage(&m)
		fmt.Println(err)
		fmt.Println(result)
		fmt.Println(m2)

		topic, errs := env.Topic(&m)
		fmt.Println(errs)
		fmt.Println(topic)

	},
}

func init() {
	RootCmd.AddCommand(testjsCmd)
}

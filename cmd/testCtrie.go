// Copyright © 2018 NAME HERE <EMAIL ADDRESS>
//
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
	"sync"

	"github.com/spf13/cobra"
	"github.com/stephane-martin/skewer/utils/ctrie/inttrie"
)

// testCtrieCmd represents the testCtrie command
var testCtrieCmd = &cobra.Command{
	Use:   "test-ctrie",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("testCtrie called")
	},
}

func init() {
	RootCmd.AddCommand(testCtrieCmd)
	trie := inttrie.New(nil)
	trie.Insert("aaa", new(int32))
	trie.Insert("bbb", new(int32))
	ptr, _ := trie.Lookup("aaa")
	if ptr != nil {
		(*ptr)++
	}
	var wg sync.WaitGroup
	for i := int32(0); i < 20; i++ {
		wg.Add(1)
		go func(j int32) {
			defer wg.Done()
			ptr := new(int32)
			*ptr = j
			trie.Insert(fmt.Sprintf("zogzog_%d", j), ptr)
		}(i)
	}
	wg.Wait()
	trie.Remove("zogzog_13")
	trie.Remove("zogzog_134")
	ch := make(chan inttrie.Entry)
	go func() {
		trie.Iterator(ch)
		close(ch)
	}()
	for e := range ch {
		fmt.Fprintln(os.Stderr, e.Key, *(e.Value))
	}

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// testCtrieCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// testCtrieCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

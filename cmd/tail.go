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
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/stephane-martin/skewer/utils/tail"
)

// tailCmd represents the tail command
var tailCmd = &cobra.Command{
	Use:   "tail",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		ctx, cancel := context.WithCancel(context.Background())
		sigchan := make(chan os.Signal, 1)
		signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigchan
			cancel()
		}()
		if len(args) == 1 {
			filename := args[0]
			output := make(chan string)
			if follow {
				go tail.FollowFile(
					ctx,
					time.Second*time.Duration(pause),
					tail.Filename(filename),
					tail.NLines(int(nbLines)),
					tail.LinesChan(output),
				)
			} else {
				err = tail.TailFile(
					ctx,
					tail.Filename(filename),
					tail.NLines(int(nbLines)),
					tail.LinesChan(output),
				)
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return
			}
			n := 1
			for l := range output {
				if printLineNumbers {
					fmt.Println(n, l)
				} else {
					fmt.Println(l)
				}
				n++
			}
		} else {
			output := make(chan tail.FileLine)
			if follow {
				go tail.FollowFiles(
					ctx,
					time.Second*time.Duration(pause),
					tail.MFilenames(args),
					tail.MNLines(int(nbLines)),
					tail.MLinesChan(output),
				)
				filename := ""
				for fl := range output {
					if filename != fl.Filename {
						filename = fl.Filename
						fmt.Println()
						fmt.Println(strings.Repeat("-", len(filename)))
						fmt.Println(filename)
						fmt.Println(strings.Repeat("-", len(filename)))
					}
					fmt.Println(fl.Line)
				}
			} else {
				tail.TailFiles(
					ctx,
					tail.MFilenames(args),
					tail.MNLines(int(nbLines)),
					tail.MLinesChan(output),
				)
				results := map[string]([]string){}
				for fl := range output {
					if _, ok := results[fl.Filename]; !ok {
						results[fl.Filename] = make([]string, 0)
					}
					results[fl.Filename] = append(results[fl.Filename], fl.Line)
				}
				filenames := make([]string, 0, len(args))
				for fname := range results {
					filenames = append(filenames, fname)
				}
				sort.Strings(filenames)
				for _, fname := range filenames {
					fmt.Println(strings.Repeat("-", len(fname)))
					fmt.Println(fname)
					fmt.Println(strings.Repeat("-", len(fname)))
					fmt.Println()
					n := 1
					for _, l := range results[fname] {
						if printLineNumbers {
							fmt.Println(n, l)
						} else {
							fmt.Println(l)
						}
						n++
					}
					fmt.Println()
				}
			}
		}
	},
}

var nbLines uint
var filename string
var printLineNumbers bool
var follow bool
var pause uint

func init() {
	RootCmd.AddCommand(tailCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// tailCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// tailCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	tailCmd.Flags().UintVarP(&nbLines, "nblines", "n", 10, "how many lines to read")
	tailCmd.Flags().BoolVarP(&printLineNumbers, "linenb", "l", false, "print line numbers")
	tailCmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow file")
	tailCmd.Flags().UintVarP(&pause, "pause", "p", 1, "pause period in seconds")
}

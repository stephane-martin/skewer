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

		if len(args) == 1 && follow && !recursive {
			filename := args[0]
			output := make(chan string)
			go tail.FollowFile(
				ctx,
				tail.SleepPeriod(time.Second*time.Duration(pause)),
				tail.Filename(filename),
				tail.NLines(int(nbLines)),
				tail.LinesChan(output),
			)

			n := 1
			for l := range output {
				if printLineNumbers {
					fmt.Println(n, l)
				} else {
					fmt.Println(l)
				}
				n++
			}
		}

		if len(args) == 1 && follow && recursive {
			dirname := args[0]
			output := make(chan tail.FileLine)
			tailor, err := tail.NewTailor(output, nil)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			} else {
				tailor.AddRecursiveDirectory(dirname, func(relname string) bool {
					return true
				})
				tailor.CloseOnContext(ctx)
				for fl := range output {
					fmt.Println(fl.Filename, fl.Line)
				}
			}
		}

		if len(args) == 1 && !follow {
			filename := args[0]
			output := make(chan string)

			err = tail.TailFile(
				ctx,
				tail.Filename(filename),
				tail.NLines(int(nbLines)),
				tail.LinesChan(output),
			)
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
		}

		if len(args) > 1 && follow {
			output := make(chan tail.FileLine)
			go tail.FollowFiles(
				ctx,
				tail.MSleepPeriod(time.Second*time.Duration(pause)),
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
		}

		if len(args) > 1 && !follow {
			output := make(chan tail.FileLine)
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
	},
}

var nbLines uint
var filename string
var printLineNumbers bool
var follow bool
var pause uint
var recursive bool

func init() {
	RootCmd.AddCommand(tailCmd)

	tailCmd.Flags().UintVarP(&nbLines, "nblines", "n", 10, "how many lines to read")
	tailCmd.Flags().BoolVarP(&printLineNumbers, "linenb", "l", false, "print line numbers")
	tailCmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow file")
	tailCmd.Flags().UintVarP(&pause, "pause", "p", 1, "pause period in seconds")
	tailCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "recursively watch directory")
}

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
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("testCtrie called")
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

	},
}

func init() {
	RootCmd.AddCommand(testCtrieCmd)
}

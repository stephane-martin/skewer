package main

import (
	"fmt"
	"os"

	"github.com/stephane-martin/skewer/cmd"
	ssys "github.com/stephane-martin/skewer/sys"
)

func main() {
	fmt.Fprintf(os.Stderr, "[arg: %s]\n", os.Args[0])
	if ssys.CapabilitiesSupported {
		ssys.Predrop()
	}
	cmd.Execute()
}

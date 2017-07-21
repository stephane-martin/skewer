package main

import (
	"fmt"
	"os"

	"github.com/stephane-martin/skewer/cmd"
	ssys "github.com/stephane-martin/skewer/sys"
)

func main() {
	prgName := os.Args[0]
	fmt.Fprintf(os.Stderr, "[program name: %s]\n", prgName)
	if ssys.CapabilitiesSupported {
		ssys.Predrop()
	}
	cmd.Execute()
}

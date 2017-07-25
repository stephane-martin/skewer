package main

import (
	"github.com/stephane-martin/skewer/cmd"
	ssys "github.com/stephane-martin/skewer/sys"
)

func main() {
	if ssys.CapabilitiesSupported {
		ssys.Predrop()
	}
	cmd.Execute()
}

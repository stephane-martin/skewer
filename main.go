package main

import "github.com/stephane-martin/skewer/cmd"
import "github.com/stephane-martin/skewer/sys"

func main() {
	if sys.CapabilitiesSupported {
		sys.Predrop()
	}
	cmd.Execute()
}

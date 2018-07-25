package main

import (
	"os"

	"github.com/concourse/atc/atccmd"
	"github.com/jessevdk/go-flags"
)

func main() {
	cmd := &atccmd.ATCCommand{}

	parser := flags.NewParser(cmd, flags.Default)
	parser.NamespaceDelimiter = "-"

	cmd.WireDynamicFlags(parser.Command.Find("run"))

	_, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	}
}

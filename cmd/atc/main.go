package main

import (
	"fmt"
	"os"

	"github.com/concourse/atc/atccmd"
	"github.com/jessevdk/go-flags"
)

func main() {
	cmd := &atccmd.ATCCommand{}

	parser := flags.NewParser(cmd, flags.Default)
	parser.NamespaceDelimiter = "-"

	cmd.WireDynamicFlags(parser.Command)

	args, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	}

	err = cmd.Execute(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

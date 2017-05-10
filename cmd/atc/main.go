package main

import (
	"fmt"
	"os"

	"github.com/concourse/atc/atccmd"
	"github.com/jessevdk/go-flags"

	_ "github.com/concourse/atc/auth/genericoauth"
	_ "github.com/concourse/atc/auth/github"
	_ "github.com/concourse/atc/auth/uaa"

	_ "github.com/concourse/atc/metric/emitter"
)

func main() {
	cmd := &atccmd.ATCCommand{}

	parser := flags.NewParser(cmd, flags.Default)
	parser.NamespaceDelimiter = "-"

	cmd.WireDynamicFlags(parser)

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

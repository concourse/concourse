package main

import (
	"fmt"
	"os"

	"github.com/concourse/concourse/worker/baggageclaim/baggageclaimcmd"
	"github.com/concourse/concourse/worker/runtime/spec"
	"github.com/jessevdk/go-flags"
)

func main() {
	cmd := &baggageclaimcmd.BaggageclaimCommand{}

	parser := flags.NewParser(cmd, flags.Default)
	parser.NamespaceDelimiter = "-"

	args, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	}

	err = cmd.Execute(args, spec.FullPrivilegedMode)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

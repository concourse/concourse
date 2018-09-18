package main

import (
	"fmt"
	_ "net/http/pprof"
	"os"

	"github.com/concourse/concourse/tsa/tsacmd"
	"github.com/jessevdk/go-flags"
)

func main() {
	cmd := &tsacmd.TSACommand{}

	parser := flags.NewParser(cmd, flags.Default)
	parser.NamespaceDelimiter = "-"

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

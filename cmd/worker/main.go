package main

import (
	"fmt"
	"os"

	"github.com/concourse/worker/drainer"
	flags "github.com/jessevdk/go-flags"
)

// overridden via linker flags
var Version = "0.0.0-dev"

type WorkerCommand struct {
	Drain drainer.Config `command:"drain" description:"Drain worker Configuration"`
	Start StartCommand   `command:"start" description:"Worker start Configuration"`
}

func main() {
	var workerCmd WorkerCommand

	parser := flags.NewParser(&workerCmd, flags.HelpFlag|flags.PassDoubleDash)
	parser.NamespaceDelimiter = "-"

	_, err := parser.Parse()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

package main

import (
	"fmt"
	"os"

	"github.com/concourse/concourse/worker/drainer"
	"github.com/concourse/concourse/worker/land"
	"github.com/concourse/concourse/worker/retire"
	"github.com/concourse/concourse/worker/start"
	flags "github.com/jessevdk/go-flags"
)

type WorkerCommand struct {
	Drain  drainer.Config             `command:"drain" description:"Drain worker Configuration"`
	Start  start.StartCommand         `command:"start" description:"Worker start Configuration"`
	Retire retire.RetireWorkerCommand `command:"retire" description:"Retire worker Configuration"`
	Land   land.LandWorkerCommand     `command:"land" description:"Land worker Configuration"`
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

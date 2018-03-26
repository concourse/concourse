package main

import (
	"github.com/concourse/worker/land"
	"github.com/concourse/worker/retire"
	flags "github.com/jessevdk/go-flags"
)

type ConcourseCommand struct {
	Version func() `short:"v" long:"version" description:"Print the version of Concourse and exit"`

	Web    WebCommand    `command:"web"    description:"Run the web UI and build scheduler."`
	Worker WorkerCommand `command:"worker" description:"Run and register a worker."`

	Quickstart QuickstartCommand `command:"quickstart" hidden:"true" description:"Run both 'web' and 'worker' together, auto-wired. Not recommended for production."`

	LandWorker   land.LandWorkerCommand     `command:"land-worker" description:"Safely drain a worker's assignments for temporary downtime."`
	RetireWorker retire.RetireWorkerCommand `command:"retire-worker" description:"Safely remove a worker from the cluster permanently."`
}

func (cmd ConcourseCommand) lessenRequirements(parser *flags.Parser) {
	cmd.Quickstart.lessenRequirements(parser.Find("quickstart"))
	cmd.Web.lessenRequirements(parser.Find("web"))
	cmd.Worker.lessenRequirements("", parser.Find("worker"))
}

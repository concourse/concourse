package commands

import (
	"fmt"
	"log"
	"os"

	"github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/atcclient/eventstream"
	"github.com/concourse/fly/rc"
)

type WatchCommand struct {
	Job   JobFlag `short:"j" long:"job"   value-name:"[PIPELINE/]JOB"   description:"Watches builds of the given job"`
	Build string  `short:"b" long:"build"                               description:"Watches a specific build"`
}

func (command *WatchCommand) Execute(args []string) error {
	client, err := rc.TargetClient(globalOptions.Target)
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	handler := atcclient.NewAtcHandler(client)

	build, err := GetBuild(handler, command.Job.JobName, command.Build, command.Job.PipelineName)
	if err != nil {
		log.Fatalln(err)
	}

	eventSource, err := handler.BuildEvents(fmt.Sprintf("%d", build.ID))

	if err != nil {
		log.Println("failed to attach to stream:", err)
		os.Exit(1)
	}

	exitCode := eventstream.Render(os.Stdout, eventSource)

	eventSource.Close()

	os.Exit(exitCode)

	return nil
}

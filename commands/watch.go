package commands

import (
	"fmt"
	"log"
	"os"

	"github.com/concourse/go-concourse/concourse"
	"github.com/concourse/go-concourse/concourse/eventstream"
	"github.com/concourse/fly/rc"
)

type WatchCommand struct {
	Job   JobFlag `short:"j" long:"job"   value-name:"[PIPELINE/]JOB"   description:"Watches builds of the given job"`
	Build string  `short:"b" long:"build"                               description:"Watches a specific build"`
}

func (command *WatchCommand) Execute(args []string) error {
	connection, err := rc.TargetConnection(Fly.Target)
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	client := concourse.NewClient(connection)

	build, err := GetBuild(client, command.Job.JobName, command.Build, command.Job.PipelineName)
	if err != nil {
		log.Fatalln(err)
	}

	eventSource, err := client.BuildEvents(fmt.Sprintf("%d", build.ID))

	if err != nil {
		log.Println("failed to attach to stream:", err)
		os.Exit(1)
	}

	exitCode := eventstream.Render(os.Stdout, eventSource)

	eventSource.Close()

	os.Exit(exitCode)

	return nil
}

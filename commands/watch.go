package commands

import (
	"fmt"
	"os"

	"github.com/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/fly/eventstream"
	"github.com/concourse/fly/rc"
)

type WatchCommand struct {
	Job   flaghelpers.JobFlag `short:"j" long:"job"   value-name:"PIPELINE/JOB"   description:"Watches builds of the given job"`
	Build string              `short:"b" long:"build"                               description:"Watches a specific build"`
}

func (command *WatchCommand) Execute(args []string) error {
	target, err := rc.LoadTarget(Fly.Target)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	client := target.Client()
	build, err := GetBuild(client, target.Team(), command.Job.JobName, command.Build, command.Job.PipelineName)
	if err != nil {
		return err
	}

	eventSource, err := client.BuildEvents(fmt.Sprintf("%d", build.ID))
	if err != nil {
		return err
	}

	exitCode := eventstream.Render(os.Stdout, eventSource)

	eventSource.Close()

	os.Exit(exitCode)

	return nil
}

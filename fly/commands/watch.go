package commands

import (
	"fmt"
	"os"
	"strconv"

	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/eventstream"
)

type WatchCommand struct {
	Job       flaghelpers.JobFlag `short:"j" long:"job"         value-name:"PIPELINE/JOB" env:"JOB" description:"Watches builds of the given job"`
	Build     string              `short:"b" long:"build"                                           description:"Watches a specific build"`
	Timestamp bool                `short:"t" long:"timestamps"                                      description:"Print with local timestamp"`
}

func (command *WatchCommand) Execute(args []string) error {
	target, err := Fly.RetrieveTarget()
	if err != nil {
		return err
	}

	var buildId int
	client := target.Client()
	if command.Job.JobName != "" || command.Build == "" {
		build, err := GetBuild(client, target.Team(), command.Job.JobName, command.Build, command.Job.PipelineName)
		if err != nil {
			return err
		}
		buildId = build.ID
	} else if command.Build != "" {
		buildId, err = strconv.Atoi(command.Build)

		if err != nil {
			return err
		}
	}

	eventSource, err := client.BuildEvents(fmt.Sprintf("%d", buildId))
	if err != nil {
		return err
	}

	renderOptions := eventstream.RenderOptions{ShowTimestamp: command.Timestamp}

	exitCode := eventstream.Render(os.Stdout, eventSource, renderOptions)

	eventSource.Close()

	os.Exit(exitCode)

	return nil
}

package commands

import (
	"fmt"

	"github.com/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/fly/rc"
)

type PauseJobCommand struct {
	Job flaghelpers.JobFlag `short:"j" long:"job"   value-name:"PIPELINE/JOB"   description:"Name of a job to pause"`
}

func (command *PauseJobCommand) Execute(args []string) error {
	client, err := rc.TargetClient(Fly.Target)
	if err != nil {
		return err
	}

	_, err = client.PauseJob(command.Job.PipelineName, command.Job.JobName)
	if err != nil {
		return err
	}

	fmt.Printf("paused '%s'\n", command.Job.JobName)

	return nil
}

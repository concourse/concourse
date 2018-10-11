package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
)

type PauseJobCommand struct {
	Job flaghelpers.JobFlag `short:"j" long:"job" required:"true" value-name:"PIPELINE/JOB" description:"Name of a job to pause"`
}

func (command *PauseJobCommand) Execute(args []string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	found, err := target.Team().PauseJob(command.Job.PipelineName, command.Job.JobName)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("%s/%s not found\n", command.Job.PipelineName, command.Job.JobName)
	}

	fmt.Printf("paused '%s'\n", command.Job.JobName)

	return nil
}

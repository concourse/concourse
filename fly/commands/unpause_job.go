package commands

import (
	"fmt"

	"github.com/concourse/concourse/v5/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/v5/fly/rc"
)

type UnpauseJobCommand struct {
	Job flaghelpers.JobFlag `short:"j" long:"job" required:"true" value-name:"PIPELINE/JOB" description:"Name of a job to unpause"`
}

func (command *UnpauseJobCommand) Execute(args []string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	found, err := target.Team().UnpauseJob(command.Job.PipelineName, command.Job.JobName)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("%s/%s not found\n", command.Job.PipelineName, command.Job.JobName)
	}

	fmt.Printf("unpaused '%s'\n", command.Job.JobName)

	return nil
}

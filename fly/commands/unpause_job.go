package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
)

type UnpauseJobCommand struct {
	Job flaghelpers.JobFlag `short:"j" long:"job" required:"true" value-name:"PIPELINE/JOB" env:"JOB" description:"Name of a job to unpause"`
}

func (command *UnpauseJobCommand) Execute(args []string) error {
	target, err := Fly.RetrieveTarget()
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

package commands

import (
	"fmt"

	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/fly/rc"
)

type TriggerJobCommand struct {
	Job flaghelpers.JobFlag `short:"j" long:"job" required:"true" value-name:"PIPELINE/JOB"   description:"Name of a job to start"`
}

func (command *TriggerJobCommand) Execute(args []string) error {
	pipelineName, jobName := command.Job.PipelineName, command.Job.JobName

	client, err := rc.TargetClient(Fly.Target)
	if err != nil {
		return err
	}

	_, err = client.CreateJobBuild(pipelineName, jobName)
	if err != nil {
		displayhelpers.FailWithErrorf("pipeline/job '%s/%s' not found\n", err, pipelineName, jobName)
	}
	fmt.Printf("started '%s/%s'\n", pipelineName, jobName)
	return nil
}

package commands

import (
	"fmt"

	"github.com/concourse/concourse/v8/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/v8/fly/rc"
	"github.com/concourse/concourse/v8/go-concourse/concourse"
)

type PauseJobCommand struct {
	Job          flaghelpers.JobFlag                `short:"j" long:"job" required:"true" value-name:"PIPELINE/JOB" description:"Name of a job to pause"`
	InstanceVars []flaghelpers.YAMLVariablePairFlag `short:"i" long:"instance-var" unquote:"false" value-name:"[NAME=YAML]" description:"Specify a YAML value for an instance variable (can be specified multiple times)"`
	Team         string                             `long:"team" description:"Name of the team to which the job belongs, if different from the target default"`
}

func (command *PauseJobCommand) Execute(args []string) error {
	jobName := command.Job.JobName
	pipelineRef := command.Job.PipelineRef
	if err := flaghelpers.ApplyInstanceVars(&pipelineRef, command.InstanceVars); err != nil {
		return err
	}

	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	var team concourse.Team
	if command.Team != "" {
		team, err = target.FindTeam(command.Team)
		if err != nil {
			return err
		}
	} else {
		team = target.Team()
	}

	found, err := team.PauseJob(pipelineRef, jobName)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("%s/%s not found on team %s\n", pipelineRef.String(), jobName, team.Name())
	}

	fmt.Printf("paused '%s'\n", jobName)

	return nil
}

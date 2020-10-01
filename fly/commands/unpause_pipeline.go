package commands

import (
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/go-concourse/concourse"
)

type UnpausePipelineCommand struct {
	Pipeline *flaghelpers.PipelineFlag `short:"p" long:"pipeline" description:"Pipeline to unpause"`
	All      bool                      `short:"a" long:"all"      description:"Unpause all pipelines"`
	Team     string                    `long:"team"              description:"Name of the team to which the pipeline belongs, if different from the target default"`
}

func (command *UnpausePipelineCommand) Validate() error {
	_, err := command.Pipeline.Validate()
	return err
}

func (command *UnpausePipelineCommand) Execute(args []string) error {
	if command.Pipeline == nil && !command.All {
		displayhelpers.Failf("one of the flags '-p, --pipeline' or '-a, --all' is required")
	}

	if command.Pipeline != nil && command.All {
		displayhelpers.Failf("only one of the flags '-p, --pipeline' or '-a, --all' is allowed")
	}

	err := command.Validate()
	if err != nil {
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

	var pipelineRefs []atc.PipelineRef
	if command.Pipeline != nil {
		pipelineRefs = []atc.PipelineRef{command.Pipeline.Ref()}
	}

	if command.All {
		pipelines, err := team.ListPipelines()
		if err != nil {
			return err
		}

		for _, pipeline := range pipelines {
			pipelineRefs = append(pipelineRefs, pipeline.Ref())
		}
	}

	for _, pipelineRef := range pipelineRefs {
		found, err := team.UnpausePipeline(pipelineRef)
		if err != nil {
			return err
		}

		if found {
			fmt.Printf("unpaused '%s'\n", pipelineRef.String())
		} else {
			displayhelpers.Failf("pipeline '%s' not found\n", pipelineRef.String())
		}
	}

	return nil
}

package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
)

type UnpausePipelineCommand struct {
	Pipeline flaghelpers.PipelineFlag `short:"p" long:"pipeline" description:"Pipeline to unpause"`
	All      bool                     `short:"a" long:"all"      description:"Unpause all pipelines"`
}

func (command *UnpausePipelineCommand) Validate() error {
	return command.Pipeline.Validate()
}

func (command *UnpausePipelineCommand) Execute(args []string) error {
	if string(command.Pipeline) == "" && !command.All {
		displayhelpers.Failf("one of the flags '-p, --pipeline' or '-a, --all' is required")
	}

	if string(command.Pipeline) != "" && command.All {
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

	var pipelineNames []string
	if string(command.Pipeline) != "" {
		pipelineNames = []string{string(command.Pipeline)}
	}

	if command.All {
		pipelines, err := target.Team().ListPipelines()
		if err != nil {
			return err
		}

		for _, pipeline := range pipelines {
			pipelineNames = append(pipelineNames, pipeline.Name)
		}
	}

	for _, pipelineName := range pipelineNames {
		found, err := target.Team().UnpausePipeline(pipelineName)
		if err != nil {
			return err
		}

		if found {
			fmt.Printf("unpaused '%s'\n", pipelineName)
		} else {
			displayhelpers.Failf("pipeline '%s' not found\n", pipelineName)
		}
	}

	return nil
}

package commands

import (
	"fmt"

	"github.com/concourse/concourse/v8/atc"
	"github.com/concourse/concourse/v8/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/v8/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/v8/fly/rc"
	"github.com/concourse/concourse/v8/go-concourse/concourse"
)

type PausePipelineCommand struct {
	Pipeline     *flaghelpers.PipelineFlag          `short:"p"   long:"pipeline" description:"Pipeline to pause"`
	InstanceVars []flaghelpers.YAMLVariablePairFlag `short:"i" long:"instance-var" unquote:"false" value-name:"[NAME=YAML]" description:"Specify a YAML value for an instance variable (can be specified multiple times)"`
	All          bool                               `short:"a"   long:"all"      description:"Pause all pipelines"`
	Team         string                             `long:"team"                 description:"Name of the team to which the pipeline belongs, if different from the target default"`
}

func (command *PausePipelineCommand) Validate() error {
	_, err := command.Pipeline.Validate()
	return err
}

func (command *PausePipelineCommand) Execute(args []string) error {
	if len(command.InstanceVars) > 0 && command.Pipeline == nil {
		return fmt.Errorf("--instance-var requires --pipeline")
	}

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

	var pipelineRefs []atc.PipelineRef
	if command.Pipeline != nil {
		pipelineRef := command.Pipeline.Ref()
		if err := flaghelpers.ApplyInstanceVars(&pipelineRef, command.InstanceVars); err != nil {
			return err
		}
		pipelineRefs = []atc.PipelineRef{pipelineRef}
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
		found, err := team.PausePipeline(pipelineRef)
		if err != nil {
			return err
		}

		if found {
			fmt.Printf("paused '%s'\n", pipelineRef.String())
		} else {
			displayhelpers.Failf("pipeline '%s' not found\n", pipelineRef.String())
		}
	}

	return nil
}

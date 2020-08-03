package commands

import (
	"errors"
	"fmt"
	"sort"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
)

var ErrMissingPipelineName = errors.New("Need to specify atleast one pipeline name")

type OrderPipelinesCommand struct {
	Alphabetical bool                       `short:"a"  long:"alphabetical" description:"Order all pipelines alphabetically"`
	Pipelines    []flaghelpers.PipelineFlag `short:"p" long:"pipeline" description:"Name of pipeline to order"`
}

func (command *OrderPipelinesCommand) Validate() (atc.OrderPipelinesRequest, error) {
	var pipelineRefs atc.OrderPipelinesRequest

	for _, p := range command.Pipelines {
		_, err := p.Validate()
		if err != nil {
			return nil, err
		}
		pipelineRefs = append(pipelineRefs, p.Ref())
	}
	return pipelineRefs, nil
}

func (command *OrderPipelinesCommand) Execute(args []string) error {
	var pipelineRefs atc.OrderPipelinesRequest

	if !command.Alphabetical && command.Pipelines == nil {
		displayhelpers.Failf("error: either --pipeline or --alphabetical are required")
	}

	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	if command.Alphabetical {
		ps, err := target.Team().ListPipelines()
		if err != nil {
			return err
		}

		for _, p := range ps {
			pipelineRefs = append(pipelineRefs, p.Ref())
		}
		sort.Sort(pipelineRefs)
	} else {
		pipelineRefs, err = command.Validate()
		if err != nil {
			return err
		}
	}

	err = target.Team().OrderingPipelines(pipelineRefs)
	if err != nil {
		displayhelpers.FailWithErrorf("failed to order pipelines", err)
	}

	fmt.Printf("ordered pipelines \n")
	for _, p := range pipelineRefs {
		fmt.Printf("  - %s \n", p.String())
	}

	return nil
}

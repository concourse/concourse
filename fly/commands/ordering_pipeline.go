package commands

import (
	"errors"
	"fmt"
	"sort"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
)

var ErrMissingPipelineName = errors.New("Need to specify atleast one pipeline name")

type OrderPipelinesCommand struct {
	Alphabetical bool                       `short:"a"  long:"alphabetical" description:"Order all pipelines alphabetically"`
	Pipelines    []flaghelpers.PipelineFlag `short:"p" long:"pipeline" description:"Name of pipeline to order"`
}

func (command *OrderPipelinesCommand) Validate() ([]string, error) {
	var pipelines []string

	for _, p := range command.Pipelines {
		_, err := p.Validate()
		if err != nil {
			return nil, err
		}
		pipelines = append(pipelines, string(p))
	}
	return pipelines, nil

}

func (command *OrderPipelinesCommand) Execute(args []string) error {
	var pipelines []string

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
			pipelines = append(pipelines, p.Name)
		}
		sort.Strings(pipelines)
	} else {
		pipelines, err = command.Validate()
		if err != nil {
			return err
		}
	}

	err = target.Team().OrderingPipelines(pipelines)
	if err != nil {
		displayhelpers.FailWithErrorf("failed to order pipelines", err)
	}

	fmt.Printf("ordered pipelines \n")
	for _, p := range pipelines {
		fmt.Printf("  - %s \n", p)
	}

	return nil
}

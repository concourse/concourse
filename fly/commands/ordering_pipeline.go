package commands

import (
	"errors"
	"fmt"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
)

var ErrMissingPipelineName = errors.New("Need to specify atleast one pipeline name")

type OrderPipelinesCommand struct {
	Pipelines []flaghelpers.PipelineFlag `short:"p" long:"pipeline" required:"true" description:"Name of pipeline to order"`
}

func (command *OrderPipelinesCommand) Validate() ([]string, error) {
	var pipelines []string

	for _, p := range command.Pipelines {
		err := p.Validate()
		if err != nil {
			return nil, err
		}
		pipelines = append(pipelines, string(p))
	}
	return pipelines, nil

}

func (command *OrderPipelinesCommand) Execute(args []string) error {
	target, err := Fly.RetrieveTarget()
	if err != nil {
		return err
	}

	pipelines, err := command.Validate()
	if err != nil {
		return err
	}

	err = target.Team().OrderingPipelines(pipelines)
	if err != nil {
		displayhelpers.FailWithErrorf("failed to order pipelines", err)
	}

	fmt.Printf("ordered pipelines \n")
	for _, p := range command.Pipelines {
		fmt.Printf("  - %s \n", p)
	}

	return nil
}

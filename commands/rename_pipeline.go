package commands

import (
	"fmt"

	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/rc"
)

type RenamePipelineCommand struct {
	Pipeline string `short:"o"  long:"old-name" required:"true"  description:"Pipeline to rename"`
	Name     string `short:"n"  long:"new-name" required:"true"  description:"Name to set as pipeline name"`
}

func (rp *RenamePipelineCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	found, err := target.Team().RenamePipeline(rp.Pipeline, rp.Name)
	if err != nil {
		return err
	}

	if !found {
		displayhelpers.Failf("pipeline '%s' not found\n", rp.Pipeline)
		return nil
	}

	fmt.Printf("pipeline successfully renamed to %s\n", rp.Name)

	return nil
}

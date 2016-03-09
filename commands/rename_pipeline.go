package commands

import (
	"fmt"

	"github.com/concourse/fly/rc"
)

type RenamePipelineCommand struct {
	Pipeline string `short:"p"  long:"pipeline" required:"true"      description:"Pipeline to rename"`
	Name     string `short:"n"  long:"name" required:"true"      description:"Name to set as pipeline name"`
}

func (rp *RenamePipelineCommand) Execute([]string) error {
	client, err := rc.TargetClient(Fly.Target)
	if err != nil {
		return err
	}

	renamed, err := client.RenamePipeline(rp.Pipeline, rp.Name)
	if err != nil {
		return fmt.Errorf("pipeline failed to rename")
	}

	if !renamed {
		return fmt.Errorf(fmt.Sprintf("pipeline %s not found", rp.Pipeline))
	}

	fmt.Printf("pipeline successfully renamed to %s", rp.Name)

	return nil
}

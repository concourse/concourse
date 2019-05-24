package commands

import (
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
)

type CheckResourceCommand struct {
	Resource flaghelpers.ResourceFlag `short:"r" long:"resource" required:"true" value-name:"PIPELINE/RESOURCE" description:"Name of a resource to check version for"`
	Version  *atc.Version             `short:"f" long:"from"                     value-name:"VERSION"           description:"Version of the resource to check from, e.g. ref:abcd or path:thing-1.2.3.tgz"`
}

func (command *CheckResourceCommand) Execute(args []string) error {
	target, err := Fly.RetrieveTarget()
	if err != nil {
		return err
	}

	var version atc.Version
	if command.Version != nil {
		version = *command.Version
	}

	found, err := target.Team().CheckResource(command.Resource.PipelineName, command.Resource.ResourceName, version)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("pipeline '%s' or resource '%s' not found\n", command.Resource.PipelineName, command.Resource.ResourceName)
	}

	fmt.Printf("checked '%s'\n", command.Resource.ResourceName)
	return nil
}

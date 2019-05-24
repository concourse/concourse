package commands

import (
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
)

type CheckResourceTypeCommand struct {
	ResourceType flaghelpers.ResourceFlag `short:"r" long:"resource-type" required:"true" value-name:"PIPELINE/RESOURCE-TYPE" env:"RESOURCE_TYPE" description:"Name of a resource-type to check"`
	Version      *atc.Version             `short:"f" long:"from"                          value-name:"VERSION"                                    description:"Version of the resource type to check from, e.g. digest:sha256@..."`
}

func (command *CheckResourceTypeCommand) Execute(args []string) error {
	target, err := Fly.RetrieveTarget()
	if err != nil {
		return err
	}

	var version atc.Version
	if command.Version != nil {
		version = *command.Version
	}

	found, err := target.Team().CheckResourceType(command.ResourceType.PipelineName, command.ResourceType.ResourceName, version)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("pipeline '%s' or resource-type '%s' not found\n", command.ResourceType.PipelineName, command.ResourceType.ResourceName)
	}

	fmt.Printf("checked '%s'\n", command.ResourceType.ResourceName)
	return nil
}

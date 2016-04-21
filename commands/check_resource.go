package commands

import (
	"fmt"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/fly/rc"
)

type CheckResourceCommand struct {
	Resource flaghelpers.ResourceFlag  `short:"r" long:"resource" required:"true" value-name:"PIPELINE/RESOURCE" description:"Name of a resource to check version for"`
	Versions []flaghelpers.VersionFlag `short:"f" long:"from" value-name:"VERSION" description:"Version of a resource to check from"`
}

func (command *CheckResourceCommand) Execute(args []string) error {
	client, err := rc.TargetClient(Fly.Target)
	if err != nil {
		return err
	}
	err = rc.ValidateClient(client, Fly.Target, false)
	if err != nil {
		return err
	}

	version := atc.Version{}
	for _, v := range command.Versions {
		version[v.Key] = v.Value
	}

	found, err := client.CheckResource(command.Resource.PipelineName, command.Resource.ResourceName, version)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("pipeline '%s' or resource '%s' not found\n", command.Resource.PipelineName, command.Resource.ResourceName)
	}

	fmt.Printf("checked '%s'\n", command.Resource.ResourceName)
	return nil
}

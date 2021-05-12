package commands

import (
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
)

type ClearResourceCacheCommand struct {
	Resource 	 flaghelpers.ResourceFlag `short:"r" long:"resource" required:"true" value-name:"PIPELINE/RESOURCE" description:"Name of a resource to clear cache"`
	Version      *atc.Version             `short:"v" long:"version"                  value-name:"VERSION"           description:"Version of the resource to check from, e.g. digest:sha256@..."`
}

func (command *ClearResourceCacheCommand) Execute(args []string) error {

	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	var version atc.Version
	if command.Version != nil {
		version = *command.Version
	}

	numRemoved, err := target.Team().ClearResourceCache(command.Resource.PipelineRef, command.Resource.ResourceName, version)

	if err != nil {
		return err
	} else {
		fmt.Printf("%d caches removed\n", numRemoved)
		return nil
	}
}
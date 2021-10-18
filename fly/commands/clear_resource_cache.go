package commands

import (
	"encoding/json"
	"fmt"

	"github.com/vito/go-interact/interact"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
)

type ClearResourceCacheCommand struct {
	Resource flaghelpers.ResourceFlag `short:"r" long:"resource" required:"true" value-name:"PIPELINE/RESOURCE" description:"Name of a resource to clear cache"`
	Version  *atc.Version             `short:"v" long:"version"                  value-name:"VERSION"           description:"Version of the resource to check from, e.g. digest:sha256@..., in case a version is not specified the command will delete all the resource caches for that resource"`
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
	warningMsg := fmt.Sprintf("!!! this will remove the resource cache(s) for `%s/%s`",
		command.Resource.PipelineRef.String(), command.Resource.ResourceName)

	if command.Version != nil {
		version = *command.Version
		versionJson, err := json.Marshal(version)
		if err != nil {
			return err
		}
		warningMsg += fmt.Sprintf(" with version: `%s`", versionJson)
	}

	warningMsg += "\n"
	fmt.Println(warningMsg)

	var confirm bool
	err = interact.NewInteraction("are you sure?").Resolve(&confirm)
	if err != nil || !confirm {
		fmt.Println("bailing out")
		return err
	}

	numRemoved, err := target.Team().ClearResourceCache(command.Resource.PipelineRef, command.Resource.ResourceName, version)

	if err != nil {
		return err
	} else {
		fmt.Printf("%d caches removed\n", numRemoved)
		return nil
	}
}

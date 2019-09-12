package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/go-concourse/concourse"
)

type PinResourceVersionCommand struct {
	Resource        flaghelpers.ResourceFlag `short:"r" long:"resource" required:"true" value-name:"PIPELINE/RESOURCE" description:"Name of the resource"`
	ResourceVersion flaghelpers.JsonFlag     `short:"v" long:"version" required:"true" description:"JSON string of at least one field of the version. The given key value pair(s) has to be an exact match but not all fields need to be provided. In the case of multiple resource versions matched, it will pin the latest one."`
}

func (command *PinResourceVersionCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	team := target.Team()

	versions, _, found, err := team.ResourceVersions(command.Resource.PipelineName, command.Resource.ResourceName, concourse.Page{}, command.ResourceVersion.Version)

	if err != nil {
		return err
	}

	if !found || len(versions) <= 0 {
		displayhelpers.Failf("could not find version matching %s", command.ResourceVersion.JsonString)
	}

	latestResourceVer := versions[0]
	pinned, err := team.PinResourceVersion(command.Resource.PipelineName, command.Resource.ResourceName, latestResourceVer.ID)

	if err != nil {
		return err
	}

	if pinned {
		fmt.Printf("pinned '%s/%s' at version id %d with %+v\n", command.Resource.PipelineName, command.Resource.ResourceName, latestResourceVer.ID, latestResourceVer.Version)
	} else {
		displayhelpers.Failf("could not pin '%s/%s', make sure the resource exists\n", command.Resource.PipelineName, command.Resource.ResourceName)
	}

	return nil
}

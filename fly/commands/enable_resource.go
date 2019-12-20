package commands

import (
	"encoding/json"
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/go-concourse/concourse"
)

type EnableResourceCommand struct {
	Resource flaghelpers.ResourceFlag `short:"r" long:"resource" required:"true" value-name:"PIPELINE/RESOURCE" description:"Name of the resource"`
	Version  *atc.Version             `short:"v" long:"version" description:"Version of the resource to enable. The given key value pair(s) has to be an exact match but not all fields are needed. In the case of multiple resource versions matched, it will enable the latest one."`
}

func (command *EnableResourceCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	team := target.Team()

	if command.Version != nil {
		versions, _, found, err := team.ResourceVersions(command.Resource.PipelineName, command.Resource.ResourceName, concourse.Page{}, *command.Version)

		if err != nil {
			return err
		}

		if !found || len(versions) <= 0 {
			enableVersionBytes, err := json.Marshal(command.Version)
			if err != nil {
				return err
			}

			displayhelpers.Failf("could not find version matching %s", string(enableVersionBytes))
		}

		latestResourceVer := versions[0]
		enabled, err := team.EnableResourceVersion(command.Resource.PipelineName, command.Resource.ResourceName, latestResourceVer.ID)

		if err != nil {
			return err
		}

		if enabled {
			versionBytes, err := json.Marshal(latestResourceVer.Version)
			if err != nil {
				return err
			}

			fmt.Printf("enabled '%s/%s' with version %s\n", command.Resource.PipelineName, command.Resource.ResourceName, string(versionBytes))
		} else {
			displayhelpers.Failf("could not enable '%s/%s', make sure the resource exists\n", command.Resource.PipelineName, command.Resource.ResourceName)
		}
	}

	return nil
}

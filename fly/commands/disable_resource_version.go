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

type DisableResourceVersionCommand struct {
	Resource flaghelpers.ResourceFlag `short:"r" long:"resource" required:"true" value-name:"PIPELINE/RESOURCE" description:"Name of the resource"`
	Version  *atc.Version             `short:"v" long:"version" required:"true" value-name:"KEY:VALUE" description:"Version of the resource to disable. The given key value pair(s) has to be an exact match but not all fields are needed. In the case of multiple resource versions matched, it will disable the latest one."`
}

func (command *DisableResourceVersionCommand) Execute([]string) error {
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
			pinVersionBytes, err := json.Marshal(command.Version)
			if err != nil {
				return err
			}

			displayhelpers.Failf("could not find version matching %s", string(pinVersionBytes))
		}

		latestResourceVer := versions[0]
		disabled := !latestResourceVer.Enabled

		if !disabled {
			disabled, err = team.DisableResourceVersion(command.Resource.PipelineName, command.Resource.ResourceName, latestResourceVer.ID)
			if err != nil {
				return err
			}
		}

		if disabled {
			versionBytes, err := json.Marshal(latestResourceVer.Version)
			if err != nil {
				return err
			}

			fmt.Printf("disabled '%s/%s' with version %s\n", command.Resource.PipelineName, command.Resource.ResourceName, string(versionBytes))
		} else {
			displayhelpers.Failf("could not disable '%s/%s', make sure the resource version exists\n", command.Resource.PipelineName, command.Resource.ResourceName)
		}
	}

	return nil
}

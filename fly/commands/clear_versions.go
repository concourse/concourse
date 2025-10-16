package commands

import (
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/jessevdk/go-flags"
	"github.com/vito/go-interact/interact"
)

type ClearVersionsCommand struct {
	Resource        flaghelpers.ResourceFlag `long:"resource" value-name:"PIPELINE/RESOURCE" description:"Name of a resource to clear versions"`
	ResourceType    flaghelpers.ResourceFlag `long:"resource-type" value-name:"PIPELINE/RESOURCE-TYPE" description:"Name of a resource type to clear versions"`
	SkipInteractive bool                     `short:"n"  long:"non-interactive"          description:"Clear resource versions or resource type versions without confirmation"`
}

func (command *ClearVersionsCommand) Execute(args []string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	if command.Resource.ResourceName == "" && command.ResourceType.ResourceName == "" {
		return &flags.Error{
			Type:    flags.ErrRequired,
			Message: "please specify one of the required flags --resource or --resource-type",
		}
	} else if command.Resource.ResourceName != "" && command.ResourceType.ResourceName != "" {
		return &flags.Error{
			Type:    flags.ErrRequired,
			Message: "can specify only one of --resource or --resource-type",
		}
	}

	team := target.Team()

	if command.Resource.ResourceName != "" {
		shared, found, err := team.ListSharedForResource(command.Resource.PipelineRef, command.Resource.ResourceName)
		if err != nil {
			return err
		}

		if !found {
			return fmt.Errorf("resource '%s' is not found", command.Resource.ResourceName)
		}

		if !command.SkipInteractive {
			confirmed, err := command.warningMessage(shared)
			if err != nil {
				return err
			}

			if !confirmed {
				return nil
			}
		}

		numDeleted, err := team.ClearResourceVersions(command.Resource.PipelineRef, command.Resource.ResourceName)
		if err != nil {
			return err
		} else {
			fmt.Printf("%d versions removed\n", numDeleted)
			return nil
		}

	} else if command.ResourceType.ResourceName != "" {
		shared, found, err := team.ListSharedForResourceType(command.ResourceType.PipelineRef, command.ResourceType.ResourceName)
		if err != nil {
			return err
		}

		if !found {
			return fmt.Errorf("resource type '%s' is not found", command.ResourceType.ResourceName)
		}

		if !command.SkipInteractive {
			confirmed, err := command.warningMessage(shared)
			if err != nil {
				return err
			}

			if !confirmed {
				return nil
			}
		}

		numDeleted, err := team.ClearResourceTypeVersions(command.ResourceType.PipelineRef, command.ResourceType.ResourceName)
		if err != nil {
			return err
		} else {
			fmt.Printf("%d versions removed\n", numDeleted)
			return nil
		}
	}

	return nil
}

func (command *ClearVersionsCommand) warningMessage(shared atc.ResourcesAndTypes) (bool, error) {
	fmt.Println("!!! this will clear the version histories for the following resources:")
	for _, r := range shared.Resources {
		fmt.Printf("- %s/%s/%s\n", r.TeamName, r.PipelineName, r.Name)
	}

	fmt.Println(`
and the following resource types:`)
	for _, rt := range shared.ResourceTypes {
		fmt.Printf("- %s/%s/%s\n", rt.TeamName, rt.PipelineName, rt.Name)
	}

	fmt.Println("")

	var confirm bool
	err := interact.NewInteraction("are you sure?").Resolve(&confirm)
	if err != nil || !confirm {
		fmt.Println("bailing out")
		return false, err
	}

	return true, nil
}

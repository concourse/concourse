package commands

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
)

type CheckResourceCommand struct {
	Resource flaghelpers.ResourceFlag `short:"r" long:"resource" required:"true" value-name:"PIPELINE/RESOURCE" description:"Name of a resource to check version for"`
	Version  *atc.Version             `short:"f" long:"from"                     value-name:"VERSION"           description:"Version of the resource to check from, e.g. ref:abcd or path:thing-1.2.3.tgz"`
	Async    bool                     `short:"a" long:"async"                    value-name:"ASYNC"             description:"Return the check without waiting for its result"`
	Shallow  bool                     `long:"shallow"                          value-name:"SHALLOW"         description:"Check the resource itself only"`
}

func (command *CheckResourceCommand) Execute(args []string) error {
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

	if !command.Shallow {
		err = command.checkParent(target)
		if err != nil {
			return err
		}
	}

	build, found, err := target.Team().CheckResource(command.Resource.PipelineName, command.Resource.ResourceName, version)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("pipeline '%s' or resource '%s' not found\n", command.Resource.PipelineName, command.Resource.ResourceName)
	}

	var buildID = strconv.Itoa(build.ID)

	if !command.Async {
		for build.Status == "started" {
			time.Sleep(time.Second)

			build, found, err = target.Client().Build(buildID)
			if err != nil {
				return err
			}

			if !found {
				return fmt.Errorf("check '%s' not found\n", buildID)
			}
		}
	}

	table := ui.Table{
		Headers: ui.TableRow{
			{Contents: "id", Color: color.New(color.Bold)},
			{Contents: "name", Color: color.New(color.Bold)},
			{Contents: "status", Color: color.New(color.Bold)},
		},
	}

	table.Data = append(table.Data, []ui.TableCell{
		{Contents: buildID},
		{Contents: command.Resource.ResourceName},
		{Contents: build.Status},
	})

	if err = table.Render(os.Stdout, Fly.PrintTableHeaders); err != nil {
		return err
	}

	if build.Status != "started" && build.Status != "succeeded" {
		os.Exit(1)
	}

	return nil
}

func (command *CheckResourceCommand) checkParent(target rc.Target) error {
	resource, found, err := target.Team().Resource(command.Resource.PipelineName, command.Resource.ResourceName)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("resource '%s' not found\n", command.Resource.ResourceName)
	}

	resourceTypes, found, err := target.Team().VersionedResourceTypes(command.Resource.PipelineName)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("pipeline '%s' not found\n", command.Resource.PipelineName)
	}

	parentType, found := command.findParent(resource, resourceTypes)
	if !found {
		return nil
	}

	cmd := &CheckResourceTypeCommand{
		ResourceType: flaghelpers.ResourceFlag{
			ResourceName: parentType.Name,
			PipelineName: command.Resource.PipelineName,
		},
	}

	return cmd.Execute(nil)
}

func (command *CheckResourceCommand) findParent(resource atc.Resource, resourceTypes atc.VersionedResourceTypes) (atc.VersionedResourceType, bool) {
	for _, t := range resourceTypes {
		if t.Name != resource.Name && t.Name == resource.Type {
			return t, true
		}
	}
	return atc.VersionedResourceType{}, false
}

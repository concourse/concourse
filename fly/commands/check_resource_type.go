package commands

import (
	"fmt"
	"os"
	"strconv"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/eventstream"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
)

type CheckResourceTypeCommand struct {
	ResourceType flaghelpers.ResourceFlag `short:"r" long:"resource-type" required:"true" value-name:"PIPELINE/RESOURCE-TYPE" description:"Name of a resource-type to check"`
	Version      *atc.Version             `short:"f" long:"from"                     value-name:"VERSION"           description:"Version of the resource type to check from, e.g. digest:sha256@..."`
	Async        bool                     `short:"a" long:"async"                    value-name:"ASYNC"             description:"Return the check without waiting for its result"`
	Shallow      bool                     `long:"shallow"                          value-name:"SHALLOW"         description:"Check the resource type itself only"`
}

func (command *CheckResourceTypeCommand) Execute(args []string) error {

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

	build, found, err := target.Team().CheckResourceType(command.ResourceType.PipelineName, command.ResourceType.ResourceName, version)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("pipeline '%s' or resource-type '%s' not found\n", command.ResourceType.PipelineName, command.ResourceType.ResourceName)
	}

	fmt.Printf("checking %s in build %d\n", ui.Embolden(command.ResourceType.String()), build.ID)

	if command.Async {
		return nil
	}

	eventSource, err := target.Client().BuildEvents(strconv.Itoa(build.ID))
	if err != nil {
		return err
	}

	renderOptions := eventstream.RenderOptions{}

	exitCode := eventstream.Render(os.Stdout, eventSource, renderOptions)
	eventSource.Close()

	if exitCode != 0 {
		os.Exit(exitCode)
	}

	return nil
}

func (command *CheckResourceTypeCommand) checkParent(target rc.Target) error {
	resourceTypes, found, err := target.Team().VersionedResourceTypes(command.ResourceType.PipelineName)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("pipeline '%s' not found\n", command.ResourceType.PipelineName)
	}

	resourceType, found := resourceTypes.Lookup(command.ResourceType.ResourceName)
	if !found {
		return fmt.Errorf("resource type '%s' not found\n", command.ResourceType.ResourceName)
	}

	parentType, found := command.findParent(resourceType.ResourceType, resourceTypes)
	if !found {
		return nil
	}

	cmd := &CheckResourceTypeCommand{
		ResourceType: flaghelpers.ResourceFlag{
			ResourceName: parentType.Name,
			PipelineName: command.ResourceType.PipelineName,
		},
	}

	err = cmd.Execute(nil)
	if err != nil {
		return err
	}

	fmt.Println()

	return nil
}

func (command *CheckResourceTypeCommand) findParent(resourceType atc.ResourceType, resourceTypes atc.VersionedResourceTypes) (atc.VersionedResourceType, bool) {
	for _, t := range resourceTypes {
		if t.Name != resourceType.Name && t.Name == resourceType.Type {
			return t, true
		}
	}
	return atc.VersionedResourceType{}, false
}

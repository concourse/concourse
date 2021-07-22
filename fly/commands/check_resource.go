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

	build, found, err := target.Team().CheckResource(command.Resource.PipelineRef, command.Resource.ResourceName, version, command.Shallow)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("pipeline '%s' or resource '%s' not found\n", command.Resource.PipelineRef.String(), command.Resource.ResourceName)
	}

	fmt.Printf("checking %s in build %d\n", ui.Embolden(command.Resource.String()), build.ID)

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


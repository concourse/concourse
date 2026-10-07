package commands

import (
	"fmt"
	"os"
	"strconv"

	"github.com/concourse/concourse/v8/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/v8/fly/eventstream"
	"github.com/concourse/concourse/v8/fly/rc"
	"github.com/concourse/concourse/v8/fly/ui"
	"github.com/concourse/concourse/v8/go-concourse/concourse"
)

type CheckPipelineResourcesCommand struct {
	Pipeline flaghelpers.PipelineFlag `short:"p" long:"pipeline" required:"true" value-name:"PIPELINE" description:"Name of the pipeline whose resources to check"`
	Async    bool                     `short:"a" long:"async"    value-name:"ASYNC"   description:"Return the checks without waiting for their result"`
	Shallow  bool                     `long:"shallow"            value-name:"SHALLOW" description:"Check the resource itself only, skip recursive resource-type checks"`
	Team     flaghelpers.TeamFlag     `long:"team"               description:"Name of the team to which the pipeline belongs, if different from the target default"`
}

func (command *CheckPipelineResourcesCommand) Execute(args []string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	var team concourse.Team
	team, err = command.Team.LoadTeam(target)
	if err != nil {
		return err
	}

	pipelineRef := command.Pipeline.Ref()

	resources, err := team.ListResources(pipelineRef)
	if err != nil {
		return err
	}

	if len(resources) == 0 {
		fmt.Println("no resources found")
		return nil
	}

	var errs []error
	maxExitCode := 0

	for _, resource := range resources {
		build, found, err := team.CheckResource(pipelineRef, resource.Name, nil, command.Shallow)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if !found {
			errs = append(errs, fmt.Errorf("pipeline '%s' or resource '%s' not found", pipelineRef.String(), resource.Name))
			continue
		}

		fmt.Printf("checking %s in build %d\n", ui.Embolden(fmt.Sprintf("%s/%s", pipelineRef, resource.Name)), build.ID)

		if command.Async {
			continue
		}

		eventSource, err := target.Client().BuildEvents(strconv.Itoa(build.ID))
		if err != nil {
			errs = append(errs, err)
			continue
		}

		renderOptions := eventstream.RenderOptions{}

		exitCode := eventstream.Render(os.Stdout, eventSource, renderOptions)
		eventSource.Close()

		if exitCode != 0 {
			if exitCode > maxExitCode {
				maxExitCode = exitCode
			}
		}
	}

	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "%s\n", e)
		}
		os.Exit(1)
	}

	if maxExitCode != 0 {
		os.Exit(maxExitCode)
	}

	return nil
}

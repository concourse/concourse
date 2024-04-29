package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
)

type RenamePipelineCommand struct {
	OldName string               `short:"o"  long:"old-name" required:"true"  description:"Existing pipeline or instance group to rename"`
	NewName string               `short:"n"    long:"new-name" required:"true"  description:"New name for the pipeline or instance group"`
	Team    flaghelpers.TeamFlag `long:"team" description:"Name of the team to which the pipeline belongs, if different from the target default"`
}

func (command *RenamePipelineCommand) Validate() error {
	if strings.Contains(command.OldName, "/") {
		return errors.New("old pipeline name cannot contain '/'")
	}
	if strings.Contains(command.NewName, "/") {
		return errors.New("new pipeline name cannot contain '/'")
	}
	return nil
}

func (command *RenamePipelineCommand) Execute([]string) error {
	err := command.Validate()
	if err != nil {
		return err
	}

	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	team := target.Team()
	if command.Team != "" {
		team, err = target.FindTeam(command.Team.Name())
		if err != nil {
			return err
		}
	}

	found, warnings, err := team.RenamePipeline(command.OldName, command.NewName)
	if err != nil {
		return err
	}

	if len(warnings) > 0 {
		displayhelpers.ShowWarnings(warnings)
	}

	if !found {
		displayhelpers.Failf("pipeline '%s' not found\n", command.OldName)
		return nil
	}

	fmt.Printf("pipeline successfully renamed to '%s'\n", command.NewName)

	return nil
}

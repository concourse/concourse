package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
)

type RenamePipelineCommand struct {
	OldName flaghelpers.PipelineFlag `short:"o"  long:"old-name" required:"true"  description:"Existing pipeline or instance group to rename"`
	NewName flaghelpers.PipelineFlag `short:"n"    long:"new-name" required:"true"  description:"New name for the pipeline or instance group"`
	Team    flaghelpers.TeamFlag     `long:"team" description:"Name of the team to which the pipeline belongs, if different from the target default"`
}

func (command *RenamePipelineCommand) Validate() error {
	_, err := command.OldName.Validate()
	if err != nil {
		return err
	}
	_, err = command.NewName.Validate()
	return err
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

	oldRef := command.OldName.Ref()
	newRef := command.NewName.Ref()

	found, warnings, err := team.RenamePipeline(oldRef, newRef)
	if err != nil {
		return err
	}

	if len(warnings) > 0 {
		displayhelpers.ShowWarnings(warnings)
	}

	if !found {
		displayhelpers.Failf("pipeline '%s' not found\n", oldRef.String())
		return nil
	}

	fmt.Printf("pipeline successfully renamed to '%s'\n", newRef.String())

	return nil
}

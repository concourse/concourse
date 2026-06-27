package commands

import (
	"fmt"
	"strconv"

	"github.com/concourse/concourse/go-concourse/concourse"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
)

type AbortBuildCommand struct {
	Job   flaghelpers.JobFlag  `short:"j" long:"job" value-name:"PIPELINE/JOB"   description:"Name of a job to cancel"`
	Build string               `short:"b" long:"build" required:"true" description:"If job is specified: build number to cancel. If job not specified: build id"`
	Team  flaghelpers.TeamFlag `long:"team" description:"Name of the team to which the pipeline belongs, if different from the target default"`
	Force bool                 `long:"force" description:"WARNING: Only use as a last resort! Try regular aborting first and wait 1min. Marks the build as aborted immediately and does not wait for the build's containers to stop."`
}

func (command *AbortBuildCommand) Execute([]string) error {
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

	var build atc.Build
	var exists bool
	if command.Job.PipelineRef.Name == "" && command.Job.JobName == "" {
		build, exists, err = target.Client().Build(command.Build)
	} else {
		build, exists, err = team.JobBuild(command.Job.PipelineRef, command.Job.JobName, command.Build)
	}
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("build does not exist")
	}

	if err := target.Client().AbortBuild(strconv.Itoa(build.ID), command.Force); err != nil {
		return err
	}

	doneMsg := "build successfully aborted"
	if command.Force {
		doneMsg = "build forcefully aborted"
	}
	fmt.Println(doneMsg)
	return nil
}

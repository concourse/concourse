package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/vito/go-interact/interact"
)

type ClearTaskCacheCommand struct {
	Job             flaghelpers.JobFlag  `short:"j" long:"job"  required:"true"  description:"Job to clear cache from"`
	StepName        string               `short:"s" long:"step"  required:"true" description:"Step name to clear cache from"`
	CachePath       string               `short:"c" long:"cache-path"  default:"" description:"Cache directory to clear out"`
	SkipInteractive bool                 `short:"n"  long:"non-interactive"          description:"Destroy the task cache(s) without confirmation"`
	Team            flaghelpers.TeamFlag `long:"team" description:"Name of the team to which the pipeline belongs, if different from the target default"`
}

func (command *ClearTaskCacheCommand) Execute([]string) error {
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

	warningMsg := fmt.Sprintf("!!! this will remove the task cache(s) for `%s/%s`, task step `%s`",
		command.Job.PipelineRef.String(), command.Job.JobName, command.StepName)
	if len(command.CachePath) > 0 {
		warningMsg += fmt.Sprintf(", at `%s`", command.CachePath)
	}
	warningMsg += "\n"
	fmt.Println(warningMsg)

	confirm := command.SkipInteractive
	if !confirm {
		err := interact.NewInteraction("are you sure?").Resolve(&confirm)
		if err != nil || !confirm {
			fmt.Println("bailing out")
			return err
		}
	}

	numRemoved, err := team.ClearTaskCache(command.Job.PipelineRef, command.Job.JobName, command.StepName, command.CachePath)

	if err != nil {
		fmt.Println(err.Error())
		return err
	} else {
		fmt.Printf("%d caches removed\n", numRemoved)
		return nil
	}
}

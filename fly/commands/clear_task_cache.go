package commands

import (
	"fmt"

	"github.com/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/fly/rc"
	"github.com/vito/go-interact/interact"
)

type ClearTaskCacheCommand struct {
	Job             flaghelpers.JobFlag `short:"j" long:"job"  required:"true"  description:"Job to clear cache from"`
	StepName        string              `short:"s" long:"step"  required:"true" description:"Step name to clear cache from"`
	CachePath       string              `short:"c" long:"cache-path"  default:"" description:"Cache directory to clear out"`
	SkipInteractive bool                `short:"n"  long:"non-interactive"          description:"Destroy the task cache(s) without confirmation"`
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

	warningMsg := fmt.Sprintf("!!! this will remove the task cache(s) for `%s/%s`, task step `%s`",
		command.Job.PipelineName, command.Job.JobName, command.StepName)
	if len(command.CachePath) > 0 {
		warningMsg += fmt.Sprintf(", at `%s`", command.CachePath)
	}
	warningMsg += "\n\n"
	fmt.Printf(warningMsg)

	confirm := command.SkipInteractive
	if !confirm {
		err := interact.NewInteraction("are you sure?").Resolve(&confirm)
		if err != nil || !confirm {
			fmt.Println("bailing out")
			return err
		}
	}

	numRemoved, err := target.Team().ClearTaskCache(command.Job.PipelineName, command.Job.JobName, command.StepName, command.CachePath)

	if err != nil {
		fmt.Println(err.Error())
		return err
	} else {
		fmt.Printf("%d caches removed\n", numRemoved)
		return nil
	}
}

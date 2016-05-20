package commands

import (
	"fmt"
	"strconv"

	"github.com/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/fly/rc"
)

type AbortBuildCommand struct {
	Job   flaghelpers.JobFlag `short:"j" long:"job"   required:"true" value-name:"PIPELINE/JOB"   description:"Name of a job to cancel"`
	Build string              `short:"b" long:"build" required:"true" description:"Name of the build to cancel"`
}

func (command *AbortBuildCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	build, exists, err := target.Team().JobBuild(command.Job.PipelineName, command.Job.JobName, command.Build)
	if err != nil {
		return fmt.Errorf("failed to get job build")
	}

	if !exists {
		return fmt.Errorf("job build does not exist")
	}

	if err := target.Client().AbortBuild(strconv.Itoa(build.ID)); err != nil {
		return fmt.Errorf("failed to abort build")
	}

	fmt.Println("build successfully aborted")
	return nil
}

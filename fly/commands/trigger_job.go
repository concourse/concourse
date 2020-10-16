package commands

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/eventstream"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/concourse/concourse/go-concourse/concourse"
)

type TriggerJobCommand struct {
	Job   flaghelpers.JobFlag `short:"j" long:"job" required:"true" value-name:"PIPELINE/JOB" description:"Name of a job to trigger"`
	Watch bool                `short:"w" long:"watch" description:"Start watching the build output"`
	Team  string              `long:"team" description:"Name of the team to which the job belongs, if different from the target default"`
}

func (command *TriggerJobCommand) Execute(args []string) error {
	jobName := command.Job.JobName
	pipelineRef := command.Job.PipelineRef

	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	var (
		build atc.Build
		team  concourse.Team
	)
	if command.Team != "" {
		team, err = target.FindTeam(command.Team)
		if err != nil {
			return err
		}
	} else {
		team = target.Team()
	}

	build, err = team.CreateJobBuild(pipelineRef, jobName)
	if err != nil {
		return err
	} else {
		fmt.Printf("started %s/%s #%s\n", pipelineRef.String(), jobName, build.Name)
	}

	if command.Watch {
		terminate := make(chan os.Signal, 1)

		go func(terminate <-chan os.Signal) {
			<-terminate
			fmt.Fprintf(ui.Stderr, "\ndetached, build is still running...\n")
			fmt.Fprintf(ui.Stderr, "re-attach to it with:\n\n")
			fmt.Fprintf(ui.Stderr, "    "+ui.Embolden(fmt.Sprintf("fly -t %s watch -j %s/%s -b %s\n\n", Fly.Target, pipelineRef.String(), jobName, build.Name)))
			os.Exit(2)
		}(terminate)

		signal.Notify(terminate, syscall.SIGINT, syscall.SIGTERM)

		fmt.Println("")
		eventSource, err := target.Client().BuildEvents(fmt.Sprintf("%d", build.ID))
		if err != nil {
			return err
		}

		renderOptions := eventstream.RenderOptions{}

		exitCode := eventstream.Render(os.Stdout, eventSource, renderOptions)

		eventSource.Close()

		os.Exit(exitCode)
	}

	return nil
}

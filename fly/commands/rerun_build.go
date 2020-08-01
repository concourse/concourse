package commands

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/eventstream"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
)

type RerunBuildCommand struct {
	Job   flaghelpers.JobFlag `short:"j" long:"job" required:"true" value-name:"PIPELINE/JOB" description:"Name of the job that you want to rerun a build for"`
	Build string              `short:"b" long:"build" required:"true" description:"The number of the build to rerun"`
	Watch bool                `short:"w" long:"watch" description:"Start watching the rerun build output"`
}

func (command *RerunBuildCommand) Execute(args []string) error {
	jobName, buildName := command.Job.JobName, command.Build
	pipelineRef := command.Job.PipelineRef

	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	build, err := target.Team().RerunJobBuild(pipelineRef, jobName, buildName)
	if err != nil {
		return err
	}
	fmt.Printf("started %s/%s #%s\n", pipelineRef.String(), jobName, build.Name)

	if command.Watch {
		terminate := make(chan os.Signal, 1)

		go func(terminate <-chan os.Signal) {
			<-terminate
			fmt.Fprintf(ui.Stderr, "\ndetached, build is still running...\n")
			fmt.Fprintf(ui.Stderr, "re-attach to it with:\n\n")
			fmt.Fprintf(ui.Stderr, "    "+ui.Embolden(fmt.Sprintf("fly -t %s watch -j %s/%s -b %s\n\n", Fly.Target, pipelineRef.QueryParams(), jobName, build.Name)))
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

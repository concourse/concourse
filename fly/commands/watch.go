package commands

import (
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/eventstream"
	"github.com/concourse/concourse/fly/rc"
)

type WatchCommand struct {
	Job       flaghelpers.JobFlag `short:"j" long:"job"         value-name:"PIPELINE/JOB"  description:"Watches builds of the given job"`
	Build     string              `short:"b" long:"build"                                  description:"Watches a specific build"`
	Url       string              `short:"u" long:"url"                                    description:"URL for the build or job to watch"`
	Timestamp bool                `short:"t" long:"timestamps"                             description:"Print with local timestamp"`
}

func (command *WatchCommand) Execute(args []string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	var buildId int
	client := target.Client()
	if command.Job.JobName != "" || command.Build == "" && command.Url == "" {
		build, err := GetBuild(client, target.Team(), command.Job.JobName, command.Build, command.Job.PipelineName)
		if err != nil {
			return err
		}
		buildId = build.ID
	} else if command.Build != "" {
		buildId, err = strconv.Atoi(command.Build)

		if err != nil {
			return err
		}
	} else if command.Url != "" {
		u, err := url.Parse(command.Url)
		if err != nil {
			return err
		}
		urlMap := parseUrlPath(u.Path)

		pipelines := urlMap["pipelines"]
		jobs := urlMap["jobs"]
		raw_build := urlMap["builds"]
		if pipelines != "" && jobs != "" {
			build, err := GetBuild(client, target.Team(), jobs, raw_build, pipelines)
			if err != nil {
				return err
			}
			buildId = build.ID
		} else if raw_build != "" {
			buildId, err = strconv.Atoi(raw_build)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("No build found in %s", command.Url)
		}
	}

	eventSource, err := client.BuildEvents(fmt.Sprintf("%d", buildId))
	if err != nil {
		return err
	}

	renderOptions := eventstream.RenderOptions{ShowTimestamp: command.Timestamp}

	exitCode := eventstream.Render(os.Stdout, eventSource, renderOptions)

	eventSource.Close()

	os.Exit(exitCode)

	return nil
}

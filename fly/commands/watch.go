package commands

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/eventstream"
	"github.com/concourse/concourse/fly/rc"
)

type WatchCommand struct {
	Job                      flaghelpers.JobFlag `short:"j" long:"job"         value-name:"PIPELINE/JOB"  description:"Watches builds of the given job"`
	Build                    string              `short:"b" long:"build"                                  description:"Watches a specific build"`
	Url                      string              `short:"u" long:"url"                                    description:"URL for the build or job to watch"`
	Timestamp                bool                `short:"t" long:"timestamps"                             description:"Print with local timestamp"`
	IgnoreEventParsingErrors bool                `long:"ignore-event-parsing-errors"                      description:"Ignore event parsing errors"`
}

func getBuildIDFromURL(target rc.Target, urlParam string) (int, error) {
	var buildId int
	client := target.Client()

	u, err := url.Parse(urlParam)
	if err != nil {
		return 0, err
	}

	urlMap := parseUrlPath(u.Path)

	parsedTargetUrl := url.URL{
		Scheme: u.Scheme,
		Host:   u.Host,
	}

	host := parsedTargetUrl.String()
	if host != target.URL() {
		err = fmt.Errorf("URL doesn't match target (%s, %s)", urlParam, target.URL())
		return 0, err
	}

	team := urlMap["teams"]
	if team != "" && team != target.Team().Name() {
		err = fmt.Errorf("Team in URL doesn't match the current team of the target (%s, %s)", urlParam, team)
		return 0, err
	}

	if urlMap["pipelines"] != "" && urlMap["jobs"] != "" {
		pipelineRef := atc.PipelineRef{Name: urlMap["pipelines"]}
		if instanceVars := u.Query().Get("instance_vars"); instanceVars != "" {
			err := json.Unmarshal([]byte(instanceVars), &pipelineRef.InstanceVars)
			if err != nil {
				err = fmt.Errorf("Failed to parse query params in (%s, %s)", urlParam, target.URL())
				return 0, err
			}
		}
		build, err := GetBuild(client, target.Team(), urlMap["jobs"], urlMap["builds"], pipelineRef)

		if err != nil {
			return 0, err
		}
		buildId = build.ID
	} else if urlMap["builds"] != "" {
		buildId, err = strconv.Atoi(urlMap["builds"])

		if err != nil {
			return 0, err
		}
	} else {
		return 0, fmt.Errorf("No build found in %s", urlParam)
	}
	return buildId, nil
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
		build, err := GetBuild(client, target.Team(), command.Job.JobName, command.Build, command.Job.PipelineRef)
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
		buildId, err = getBuildIDFromURL(target, command.Url)

		if err != nil {
			return err
		}
	}

	eventSource, err := client.BuildEvents(fmt.Sprintf("%d", buildId))
	if err != nil {
		return err
	}

	renderOptions := eventstream.RenderOptions{
		ShowTimestamp:            command.Timestamp,
		IgnoreEventParsingErrors: command.IgnoreEventParsingErrors,
	}

	exitCode := eventstream.Render(os.Stdout, eventSource, renderOptions)

	eventSource.Close()

	os.Exit(exitCode)

	return nil
}

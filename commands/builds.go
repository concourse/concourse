package commands

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/concourse/go-concourse/concourse"
	"github.com/fatih/color"
)

const timeDateLayout = "2006-01-02@15:04:05-0700"

type BuildsCommand struct {
	Count    int                      `short:"c" long:"count" default:"50" description:"Number of builds you want to limit the return to"`
	Job      flaghelpers.JobFlag      `short:"j" long:"job" value-name:"PIPELINE/JOB" description:"Name of a job to get builds for"`
	Pipeline flaghelpers.PipelineFlag `short:"p" long:"pipeline" description:"Name of a pipeline to get builds for"`
	Team     bool                     `short:"t"  long:"team" description:"Only show builds for the currently targeted team"`
	Json     bool                     `long:"json" description:"Print command result as JSON"`
}

func (command *BuildsCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	page := concourse.Page{Limit: command.Count}

	team := target.Team()
	client := target.Client()

	var builds []atc.Build
	if command.pipelineFlag() && command.jobFlag() {
		return errors.New("Cannot specify both --pipeline and --job")
	} else if command.pipelineFlag() {
		err = command.Pipeline.Validate()
		if err != nil {
			return err
		}

		var found bool
		builds, _, found, err = team.PipelineBuilds(
			string(command.Pipeline),
			page,
		)
		if err != nil {
			return err
		}

		if !found {
			displayhelpers.Failf("pipeline not found")
		}
	} else if command.jobFlag() {
		var found bool
		builds, _, found, err = team.JobBuilds(
			command.Job.PipelineName,
			command.Job.JobName,
			page,
		)
		if err != nil {
			return err
		}

		if !found {
			displayhelpers.Failf("pipeline/job not found")
		}
	} else if command.Team {
		builds, _, err = team.Builds(page)
		if err != nil {
			return err
		}
	} else {
		builds, _, err = client.Builds(page)
		if err != nil {
			return err
		}
	}

	if command.Json {
		err = displayhelpers.JsonPrint(builds)
		if err != nil {
			return err
		}
		return nil
	}

	table := ui.Table{
		Headers: ui.TableRow{
			{Contents: "id", Color: color.New(color.Bold)},
			{Contents: "pipeline/job", Color: color.New(color.Bold)},
			{Contents: "build", Color: color.New(color.Bold)},
			{Contents: "status", Color: color.New(color.Bold)},
			{Contents: "start", Color: color.New(color.Bold)},
			{Contents: "end", Color: color.New(color.Bold)},
			{Contents: "duration", Color: color.New(color.Bold)},
		},
	}

	var rangeUntil int
	if command.Count < len(builds) {
		rangeUntil = command.Count
	} else {
		rangeUntil = len(builds)
	}

	for _, b := range builds[:rangeUntil] {
		startTimeCell, endTimeCell, durationCell := populateTimeCells(time.Unix(b.StartTime, 0), time.Unix(b.EndTime, 0))

		var pipelineJobCell, buildCell ui.TableCell
		if b.PipelineName == "" {
			pipelineJobCell.Contents = "one-off"
			buildCell.Contents = "n/a"
		} else {
			pipelineJobCell.Contents = fmt.Sprintf("%s/%s", b.PipelineName, b.JobName)
			buildCell.Contents = b.Name
		}

		var statusCell ui.TableCell
		statusCell.Contents = b.Status

		switch b.Status {
		case "pending":
			statusCell.Color = ui.PendingColor
		case "started":
			statusCell.Color = ui.StartedColor
		case "succeeded":
			statusCell.Color = ui.SucceededColor
		case "failed":
			statusCell.Color = ui.FailedColor
		case "errored":
			statusCell.Color = ui.ErroredColor
		case "aborted":
			statusCell.Color = ui.AbortedColor
		case "paused":
			statusCell.Color = ui.PausedColor
		}

		table.Data = append(table.Data, []ui.TableCell{
			{Contents: strconv.Itoa(b.ID)},
			pipelineJobCell,
			buildCell,
			statusCell,
			startTimeCell,
			endTimeCell,
			durationCell,
		})
	}

	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

func populateTimeCells(startTime time.Time, endTime time.Time) (ui.TableCell, ui.TableCell, ui.TableCell) {
	var startTimeCell ui.TableCell
	var endTimeCell ui.TableCell
	var durationCell ui.TableCell

	startTime = startTime.Local()
	endTime = endTime.Local()
	zeroTime := time.Unix(0, 0)

	if startTime == zeroTime {
		startTimeCell.Contents = "n/a"
	} else {
		startTimeCell.Contents = startTime.Format(timeDateLayout)
	}

	if endTime == zeroTime {
		endTimeCell.Contents = "n/a"
		durationCell.Contents = fmt.Sprintf("%v+", roundSecondsOffDuration(time.Since(startTime)))
	} else {
		endTimeCell.Contents = endTime.Format(timeDateLayout)
		durationCell.Contents = endTime.Sub(startTime).String()
	}

	if startTime == zeroTime && endTime == zeroTime {
		durationCell.Contents = "n/a"
	}

	return startTimeCell, endTimeCell, durationCell
}

func roundSecondsOffDuration(d time.Duration) time.Duration {
	return d - (d % time.Second)
}

func (command *BuildsCommand) jobFlag() bool {
	return command.Job.PipelineName != "" && command.Job.JobName != ""
}

func (command *BuildsCommand) pipelineFlag() bool {
	return command.Pipeline != ""
}

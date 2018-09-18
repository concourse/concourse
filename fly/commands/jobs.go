package commands

import (
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/fatih/color"
)

type JobsCommand struct {
	Pipeline string `short:"p" long:"pipeline" required:"true" description:"Get jobs in this pipeline"`
	Json     bool   `long:"json" description:"Print command result as JSON"`
}

func (command *JobsCommand) Execute([]string) error {
	pipelineName := command.Pipeline

	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	var headers []string
	var jobs []atc.Job

	jobs, err = target.Team().ListJobs(pipelineName)
	if err != nil {
		return err
	}

	if command.Json {
		err = displayhelpers.JsonPrint(jobs)
		if err != nil {
			return err
		}
		return nil
	}

	headers = []string{"name", "paused", "status", "next"}
	table := ui.Table{Headers: ui.TableRow{}}
	for _, h := range headers {
		table.Headers = append(table.Headers, ui.TableCell{Contents: h, Color: color.New(color.Bold)})
	}

	for _, p := range jobs {
		var pausedColumn ui.TableCell
		if p.Paused {
			pausedColumn.Contents = "yes"
			pausedColumn.Color = color.New(color.FgCyan)
		} else {
			pausedColumn.Contents = "no"
		}

		row := ui.TableRow{}
		row = append(row, ui.TableCell{Contents: p.Name})

		row = append(row, pausedColumn)

		var statusColumn ui.TableCell
		if p.FinishedBuild != nil {
			statusColumn.Contents = p.FinishedBuild.Status
			switch p.FinishedBuild.Status {
			case "pending":
				statusColumn.Color = ui.PendingColor
			case "started":
				statusColumn.Color = ui.StartedColor
			case "succeeded":
				statusColumn.Color = ui.SucceededColor
			case "failed":
				statusColumn.Color = ui.FailedColor
			case "errored":
				statusColumn.Color = ui.ErroredColor
			case "aborted":
				statusColumn.Color = ui.AbortedColor
			case "paused":
				statusColumn.Color = ui.PausedColor
			}
		} else {
			statusColumn.Contents = "n/a"
		}
		row = append(row, statusColumn)

		var nextColumn ui.TableCell
		if p.NextBuild != nil {
			nextColumn.Contents = p.NextBuild.Status
			switch p.NextBuild.Status {
			case "pending:":
				nextColumn.Color = ui.PendingColor
			case "started":
				nextColumn.Color = ui.StartedColor
			}
		} else {
			nextColumn.Contents = "n/a"
		}
		row = append(row, nextColumn)

		table.Data = append(table.Data, row)
	}

	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

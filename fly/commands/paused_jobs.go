package commands

import (
	"os"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/fatih/color"
)

type PausedJobsCommand struct {
	Pipeline flaghelpers.PipelineFlag `short:"p" long:"pipeline" required:"true" description:"Get jobs in this pipeline"`
	Json     bool                     `long:"json" description:"Print command result as JSON"`
	Team     flaghelpers.TeamFlag     `long:"team" description:"Name of the team to which the pipeline belongs, if different from the target default"`
}

func (command *PausedJobsCommand) Execute([]string) error {
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

	var jobs []atc.Job
	jobs, err = team.ListJobs(command.Pipeline.Ref())
	if err != nil {
		return err
	}

	jobs = command.filter(jobs)

	if command.Json {
		err = displayhelpers.JsonPrint(jobs)
		if err != nil {
			return err
		}
		return nil
	} else {
		return command.render(jobs)
	}
}

func (command *PausedJobsCommand) render(jobs []atc.Job) error {
	headers := []string{"name", "paused", "paused_by", "paused_at"}
	table := ui.Table{Headers: ui.TableRow{}}
	for _, h := range headers {
		table.Headers = append(table.Headers, ui.TableCell{Contents: h, Color: color.New(color.Bold)})
	}

	for _, j := range jobs {
		var pausedColumn ui.TableCell
		if j.Paused {
			pausedColumn.Contents = "yes"
		} else {
			pausedColumn.Contents = "no"
		}

		var pausedByColumn ui.TableCell
		if j.PausedBy == "" {
			pausedByColumn.Contents = "n/a"
		} else {
			pausedByColumn.Contents = j.PausedBy
		}

		var pausedAtColumn ui.TableCell
		if j.PausedAt == 0 {
			pausedAtColumn.Contents = "n/a"
		} else {
			pausedAtColumn.Contents = time.Unix(j.PausedAt, 0).String()
		}

		row := ui.TableRow{}
		row = append(row, ui.TableCell{Contents: j.Name})
		row = append(row, pausedColumn)
		row = append(row, pausedByColumn)
		row = append(row, pausedAtColumn)
		table.Data = append(table.Data, row)
	}

	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

func (command *PausedJobsCommand) filter(jobs []atc.Job) []atc.Job {
	pausedJobs := make([]atc.Job, 0)
	for _, j := range jobs {
		if j.Paused {
			pausedJobs = append(pausedJobs, j)
		}
	}
	return pausedJobs
}

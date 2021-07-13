package commands

import (
	"os"
	"strconv"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
)

type PausedPipelinesCommand struct {
	All  bool `short:"a"  long:"all" description:"Show pipelines across all teams"`
	Json bool `long:"json" description:"Print command result as JSON"`
}

func (command *PausedPipelinesCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	var unfilteredPipelines []atc.Pipeline

	if command.All {
		unfilteredPipelines, err = target.Client().ListPipelines()
	} else {
		unfilteredPipelines, err = target.Team().ListPipelines()
	}
	if err != nil {
		return err
	}

	headers := command.buildHeader()
	pipelines := command.filterPipelines(unfilteredPipelines)

	if command.Json {
		err = displayhelpers.JsonPrint(pipelines)
		if err != nil {
			return err
		}
		return nil
	}

	table := ui.Table{Headers: ui.TableRow{}}
	for _, h := range headers {
		table.Headers = append(table.Headers, ui.TableCell{Contents: h, Color: color.New(color.Bold)})
	}

	for _, p := range pipelines {
		var pausedColumn ui.TableCell
		if p.Paused {
			pausedColumn.Contents = "yes"
		} else {
			pausedColumn.Contents = "no"
		}

		var pausedByColumn ui.TableCell
		if p.PausedBy == "" {
			pausedByColumn.Contents = "n/a"
		} else {
			pausedByColumn.Contents = p.PausedBy
		}

		var pausedAtColumn ui.TableCell
		if p.PausedAt == 0 {
			pausedAtColumn.Contents = "n/a"
		} else {
			pausedAtColumn.Contents = time.Unix(p.PausedAt, 0).String()
		}

		row := ui.TableRow{}
		row = append(row, ui.TableCell{Contents: strconv.Itoa(p.ID)})
		row = append(row, ui.TableCell{Contents: p.Ref().String()})

		if command.All {
			row = append(row, ui.TableCell{Contents: p.TeamName})
		}

		row = append(row, pausedColumn)
		row = append(row, pausedByColumn)
		row = append(row, pausedAtColumn)
		table.Data = append(table.Data, row)
	}

	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

func (command *PausedPipelinesCommand) buildHeader() []string {
	var headers []string
	if command.All {
		headers = []string{"id", "name", "team", "paused", "paused_by", "paused_at"}
	} else {
		headers = []string{"id", "name", "paused", "paused_by", "paused_at"}
	}

	return headers
}

func (command *PausedPipelinesCommand) filterPipelines(unfilteredPipelines []atc.Pipeline) []atc.Pipeline {
	pipelines := make([]atc.Pipeline, 0)

	for _, p := range unfilteredPipelines {
		if p.Paused {
			pipelines = append(pipelines, p)
		}
	}

	return pipelines
}

package commands

import (
	"os"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
)

type PipelinesCommand struct {
	All  bool `short:"a"  long:"all" description:"Show pipelines across all teams"`
	Json bool `long:"json" description:"Print command result as JSON"`
}

func (command *PipelinesCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	var headers []string
	var unfilteredPipelines []atc.Pipeline

	if command.All {
		unfilteredPipelines, err = target.Client().ListPipelines()
		headers = []string{"name", "team", "paused", "public", "last updated"}
	} else {
		unfilteredPipelines, err = target.Team().ListPipelines()
		headers = []string{"name", "paused", "public", "last updated"}
	}
	if err != nil {
		return err
	}

	pipelines := []atc.Pipeline{}
	for _, p := range unfilteredPipelines {
		if !p.Archived {
			pipelines = append(pipelines, p)
		}
	}

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
			pausedColumn.Color = ui.OnColor
		} else {
			pausedColumn.Contents = "no"
		}

		var publicColumn ui.TableCell
		if p.Public {
			publicColumn.Contents = "yes"
			publicColumn.Color = ui.OnColor
		} else {
			publicColumn.Contents = "no"
		}

		row := ui.TableRow{}
		row = append(row, ui.TableCell{Contents: p.Name})
		if command.All {
			row = append(row, ui.TableCell{Contents: p.TeamName})
		}
		row = append(row, pausedColumn)
		row = append(row, publicColumn)
		row = append(row, ui.TableCell{Contents: time.Unix(p.LastUpdated, 0).String()})

		table.Data = append(table.Data, row)
	}

	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

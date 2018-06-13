package commands

import (
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/fatih/color"
)

type PipelinesCommand struct {
	All  bool `short:"a"  long:"all" description:"Show all pipelines"`
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
	var pipelines []atc.Pipeline

	if command.All {
		pipelines, err = target.Client().ListPipelines()
		headers = []string{"name", "team", "paused", "public"}
	} else {
		pipelines, err = target.Team().ListPipelines()
		headers = []string{"name", "paused", "public"}
	}
	if err != nil {
		return err
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
			pausedColumn.Color = color.New(color.FgCyan)
		} else {
			pausedColumn.Contents = "no"
		}

		var publicColumn ui.TableCell
		if p.Public {
			publicColumn.Contents = "yes"
			publicColumn.Color = color.New(color.FgCyan)
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

		table.Data = append(table.Data, row)
	}

	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

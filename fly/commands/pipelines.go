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
	All             bool `short:"a"  long:"all" description:"Show pipelines across all teams"`
	IncludeArchived bool `long:"include-archived" description:"Show archived pipelines"`
	Json            bool `long:"json" description:"Print command result as JSON"`
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

		var archivedColumn ui.TableCell
		if command.IncludeArchived {
			if p.Archived {
				archivedColumn.Contents = "yes"
				archivedColumn.Color = ui.OnColor
			} else {
				archivedColumn.Contents = "no"
			}
		}

		row := ui.TableRow{}
		row = append(row, ui.TableCell{Contents: p.Name})
		if command.All {
			row = append(row, ui.TableCell{Contents: p.TeamName})
		}
		row = append(row, pausedColumn)
		row = append(row, publicColumn)
		if command.IncludeArchived {
			row = append(row, archivedColumn)
		}
		row = append(row, ui.TableCell{Contents: time.Unix(p.LastUpdated, 0).String()})

		table.Data = append(table.Data, row)
	}

	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

func (command *PipelinesCommand) buildHeader() []string {
	var headers []string
	if command.All {
		headers = []string{"name", "team", "paused", "public"}
	} else {
		headers = []string{"name", "paused", "public"}
	}

	if command.IncludeArchived {
		headers = append(headers, "archived")
	}
	headers = append(headers, "last updated")

	return headers
}

func (command *PipelinesCommand) filterPipelines(unfilteredPipelines []atc.Pipeline) []atc.Pipeline {
	pipelines := make([]atc.Pipeline, 0)

	if !command.IncludeArchived {
		for _, p := range unfilteredPipelines {
			if !p.Archived {
				pipelines = append(pipelines, p)
			}
		}
	} else {
		pipelines = unfilteredPipelines
	}

	return pipelines
}

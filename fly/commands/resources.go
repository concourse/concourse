package commands

import (
	"os"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
)

type ResourcesCommand struct {
	Pipeline string `short:"p" long:"pipeline" required:"true" description:"Get resources in this pipeline"`
	Json     bool   `long:"json" description:"Print command result as JSON"`
}

func (command *ResourcesCommand) Execute([]string) error {
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
	var resources []atc.Resource

	resources, err = target.Team().ListResources(pipelineName)
	if err != nil {
		return err
	}

	if command.Json {
		err = displayhelpers.JsonPrint(resources)
		if err != nil {
			return err
		}
		return nil
	}

	headers = []string{"name", "paused", "type"}
	table := ui.Table{Headers: ui.TableRow{}}
	for _, h := range headers {
		table.Headers = append(table.Headers, ui.TableCell{Contents: h, Color: color.New(color.Bold)})
	}

	for _, p := range resources {
		var pausedColumn ui.TableCell
		if p.Paused {
			pausedColumn.Contents = "yes"
			pausedColumn.Color = ui.OnColor
		} else {
			pausedColumn.Contents = "no"
		}

		var resourceType ui.TableCell
		resourceType.Contents = p.Type

		row := ui.TableRow{}
		row = append(row, ui.TableCell{Contents: p.Name})
		row = append(row, pausedColumn)
		row = append(row, resourceType)

		table.Data = append(table.Data, row)
	}

	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

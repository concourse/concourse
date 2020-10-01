package commands

import (
	"os"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
)

type ResourcesCommand struct {
	Pipeline flaghelpers.PipelineFlag `short:"p" long:"pipeline" required:"true" description:"Get resources in this pipeline"`
	Json     bool                     `long:"json" description:"Print command result as JSON"`
}

func (command *ResourcesCommand) Execute([]string) error {
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

	resources, err = target.Team().ListResources(command.Pipeline.Ref())
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

	headers = []string{"name", "type", "pinned"}
	table := ui.Table{Headers: ui.TableRow{}}
	for _, h := range headers {
		table.Headers = append(table.Headers, ui.TableCell{Contents: h, Color: color.New(color.Bold)})
	}

	for _, p := range resources {
		row := ui.TableRow{}
		row = append(row, ui.TableCell{Contents: p.Name})
		row = append(row, ui.TableCell{Contents: p.Type})

		var pinnedColumn ui.TableCell
		if p.PinnedVersion != nil {
			pinnedColumn.Contents = ui.PresentVersion(p.PinnedVersion)
		} else {
			pinnedColumn.Contents = "n/a"
		}

		row = append(row, pinnedColumn)

		table.Data = append(table.Data, row)
	}

	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

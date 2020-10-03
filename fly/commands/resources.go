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

	headers = []string{"name", "type", "pinned", "status"}
	table := ui.Table{Headers: ui.TableRow{}}
	for _, h := range headers {
		table.Headers = append(table.Headers, ui.TableCell{Contents: h, Color: color.New(color.Bold)})
	}

	for _, resource := range resources {
		var pinnedColumn ui.TableCell
		if resource.PinnedVersion != nil {
			pinnedColumn.Contents = ui.PresentVersion(resource.PinnedVersion)
		} else {
			pinnedColumn.Contents = "n/a"
		}

		var statusColumn ui.TableCell
		if resource.FailingToCheck {
			if resource.CheckError != "" {
				statusColumn.Contents = resource.CheckError
				statusColumn.Color = ui.FailedColor
			} else if resource.CheckSetupError != "" {
				statusColumn.Contents = resource.CheckSetupError
				statusColumn.Color = ui.ErroredColor
			}
		} else {
			statusColumn.Contents = "ok"
			statusColumn.Color = ui.SucceededColor
		}

		table.Data = append(table.Data, ui.TableRow{
			ui.TableCell{Contents: resource.Name},
			ui.TableCell{Contents: resource.Type},
			pinnedColumn,
			statusColumn,
		})
	}

	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

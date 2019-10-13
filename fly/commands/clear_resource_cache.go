package commands

import (
	"os"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
)

type ClearResourceCacheCommand struct {
	Resource flaghelpers.ResourceFlag `short:"r" long:"resource-type" required:"true" value-name:"PIPELINE/RESOURCE-TYPE" description:"Name of a resource to invalidate"`
	Version  *atc.Version             `short:"f" long:"from" required:"false" value-name:"VERSION" description:"Version of the resource cache to invalidate, e.g. digest:sha256@..."`
}

func (command *ClearResourceCacheCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	var version atc.Version
	if command.Version != nil {
		version = *command.Version
	}

	result, err := target.Team().ClearResourceCache(
		command.Resource.PipelineName, command.Resource.ResourceName, version,
	)

	if err != nil {
		return err
	}

	table := ui.Table{Headers: ui.TableRow{ui.TableCell{Contents: "Versions", Color: color.New(color.Bold)}}}

	for _, version := range result {
		row := ui.TableRow{
			ui.TableCell{
				Contents: version,
				Color:    color.New(color.Italic),
			},
		}
		table.Data = append(table.Data, row)
	}

	table.Render(os.Stdout, Fly.PrintTableHeaders)

	return nil
}

package commands

import (
	"os"
	"sort"

	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/fatih/color"
)

type TeamsCommand struct {
	Json bool `long:"json" description:"Print command result as JSON"`
}

func (command *TeamsCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	teams, err := target.Client().ListTeams()
	if err != nil {
		return err
	}

	if command.Json {
		err = displayhelpers.JsonPrint(teams)
		if err != nil {
			return err
		}
		return nil
	}

	table := ui.Table{
		Headers: ui.TableRow{
			{Contents: "name", Color: color.New(color.Bold)},
		},
	}

	for _, t := range teams {
		row := ui.TableRow{
			{Contents: t.Name},
		}

		table.Data = append(table.Data, row)
	}

	sort.Sort(table.Data)

	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

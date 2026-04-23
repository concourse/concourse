package commands

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
)

type GetComponentsCommand struct {
	Json bool `long:"json" description:"Print command result as JSON"`
}

func (command *GetComponentsCommand) Execute(args []string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	components, err := target.Client().ListComponents()
	if err != nil {
		return err
	}

	slices.SortFunc(components, func(a, b atc.Component) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	if command.Json {
		err = displayhelpers.JsonPrint(components)
		if err != nil {
			return err
		}
		return nil
	}

	err = command.tableFor(components).Render(os.Stdout, Fly.PrintTableHeaders)
	if err != nil {
		return err
	}

	return nil
}

func (command *GetComponentsCommand) tableFor(components []atc.Component) ui.Table {
	headers := ui.TableRow{
		{Contents: "name", Color: color.New(color.Bold)},
		{Contents: "interval", Color: color.New(color.Bold)},
		{Contents: "last ran", Color: color.New(color.Bold)},
		{Contents: "paused", Color: color.New(color.Bold)},
	}

	table := ui.Table{Headers: headers}

	for _, c := range components {
		row := ui.TableRow{
			{Contents: c.Name},
			{Contents: c.Interval.String()},
			{Contents: c.LastRan.Local().Format(time.DateTime)},
			{Contents: fmt.Sprintf("%t", c.Paused)},
		}
		table.Data = append(table.Data, row)
	}

	return table
}

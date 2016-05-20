package commands

import (
	"os"

	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/fatih/color"
)

type PipelinesCommand struct{}

func (command *PipelinesCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	pipelines, err := target.Team().ListPipelines()
	if err != nil {
		return err
	}

	table := ui.Table{
		Headers: ui.TableRow{
			{Contents: "name", Color: color.New(color.Bold)},
			{Contents: "paused", Color: color.New(color.Bold)},
		},
	}

	for _, p := range pipelines {
		var pausedColumn ui.TableCell
		if p.Paused {
			pausedColumn.Contents = "yes"
			pausedColumn.Color = color.New(color.FgCyan)
		} else {
			pausedColumn.Contents = "no"
		}

		table.Data = append(table.Data, []ui.TableCell{
			{Contents: p.Name},
			pausedColumn,
		})
	}

	return table.Render(os.Stdout)
}

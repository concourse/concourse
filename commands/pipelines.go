package commands

import (
	"os"

	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/fatih/color"
)

type PipelinesCommand struct{}

func (command *PipelinesCommand) Execute([]string) error {
	client, err := rc.TargetClient(Fly.Target)
	if err != nil {
		return err
	}
	err = rc.ValidateClient(client)
	if err != nil {
		return err
	}

	pipelines, err := client.ListPipelines()
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

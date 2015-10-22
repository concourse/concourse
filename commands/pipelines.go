package commands

import (
	"log"
	"os"

	"github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/fatih/color"
)

type PipelinesCommand struct{}

var pipelinesCommand PipelinesCommand

func init() {
	command, err := Parser.AddCommand(
		"pipelines",
		"Print the configured pipelines",
		"",
		&pipelinesCommand,
	)
	if err != nil {
		panic(err)
	}

	command.Aliases = []string{"ps"}
}

func (command *PipelinesCommand) Execute([]string) error {
	target, err := rc.SelectTarget(globalOptions.Target, globalOptions.Insecure)
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	client, err := atcclient.NewClient(*target)
	if err != nil {
		log.Fatalln(err)
	}
	handler := atcclient.NewAtcHandler(client)

	pipelines, err := handler.ListPipelines()
	if err != nil {
		log.Fatalln(err)
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

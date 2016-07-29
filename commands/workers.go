package commands

import (
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/concourse/atc"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/fatih/color"
)

type WorkersCommand struct {
	Details bool `short:"d" long:"details" description:"Print additional information for each worker"`
}

func (command *WorkersCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	workers, err := target.Client().ListWorkers()
	if err != nil {
		return err
	}

	headers := ui.TableRow{
		{Contents: "name", Color: color.New(color.Bold)},
		{Contents: "containers", Color: color.New(color.Bold)},
		{Contents: "platform", Color: color.New(color.Bold)},
		{Contents: "tags", Color: color.New(color.Bold)},
		{Contents: "team", Color: color.New(color.Bold)},
	}

	if command.Details {
		headers = append(headers,
			ui.TableCell{Contents: "garden address", Color: color.New(color.Bold)},
			ui.TableCell{Contents: "baggageclaim url", Color: color.New(color.Bold)},
			ui.TableCell{Contents: "resource types", Color: color.New(color.Bold)},
		)
	}

	table := ui.Table{Headers: headers}

	sort.Sort(byWorkerName(workers))

	for _, w := range workers {
		row := ui.TableRow{
			{Contents: w.Name},
			{Contents: strconv.Itoa(w.ActiveContainers)},
			{Contents: w.Platform},
			stringOrDefault(strings.Join(w.Tags, ", ")),
			stringOrDefault(w.Team),
		}

		if command.Details {
			var resourceTypes []string
			for _, t := range w.ResourceTypes {
				resourceTypes = append(resourceTypes, t.Type)
			}

			row = append(row, ui.TableCell{Contents: w.GardenAddr})
			row = append(row, stringOrDefault(w.BaggageclaimURL))
			row = append(row, stringOrDefault(strings.Join(resourceTypes, ", ")))
		}

		table.Data = append(table.Data, row)
	}

	return table.Render(os.Stdout)
}

type byWorkerName []atc.Worker

func (ws byWorkerName) Len() int               { return len(ws) }
func (ws byWorkerName) Swap(i int, j int)      { ws[i], ws[j] = ws[j], ws[i] }
func (ws byWorkerName) Less(i int, j int) bool { return ws[i].Name < ws[j].Name }

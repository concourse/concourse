package commands

import (
	"log"
	"os"
	"sort"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/fatih/color"
)

type ContainersCommand struct{}

func (command *ContainersCommand) Execute([]string) error {
	client, err := rc.TargetClient(globalOptions.Target)
	if err != nil {
		log.Fatalln(err)
	}

	handler := atcclient.NewAtcHandler(client)

	containers, err := handler.ListContainers(map[string]string{})
	if err != nil {
		log.Fatalln(err)
	}

	table := ui.Table{
		Headers: ui.TableRow{
			{Contents: "handle", Color: color.New(color.Bold)},
			{Contents: "name", Color: color.New(color.Bold)},
			{Contents: "pipeline", Color: color.New(color.Bold)},
			{Contents: "type", Color: color.New(color.Bold)},
			{Contents: "build id", Color: color.New(color.Bold)},
			{Contents: "worker", Color: color.New(color.Bold)},
		},
	}

	sort.Sort(containersByHandle(containers))

	for _, c := range containers {
		row := ui.TableRow{
			{Contents: c.ID},
			{Contents: c.Name},
			{Contents: c.PipelineName},
			{Contents: c.Type},
			buildIDOrNone(c.BuildID),
			{Contents: c.WorkerName},
		}

		table.Data = append(table.Data, row)
	}

	return table.Render(os.Stdout)
}

type containersByHandle []atc.Container

func (cs containersByHandle) Len() int               { return len(cs) }
func (cs containersByHandle) Swap(i int, j int)      { cs[i], cs[j] = cs[j], cs[i] }
func (cs containersByHandle) Less(i int, j int) bool { return cs[i].ID < cs[j].ID }

func buildIDOrNone(id int) ui.TableCell {
	var column ui.TableCell

	if id == 0 {
		column.Contents = "none"
		column.Color = color.New(color.Faint)
	} else {
		column.Contents = strconv.Itoa(id)
	}

	return column
}

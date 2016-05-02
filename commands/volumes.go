package commands

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/fatih/color"
)

type VolumesCommand struct{}

func (command *VolumesCommand) Execute([]string) error {
	client, err := rc.TargetClient(Fly.Target)
	if err != nil {
		return err
	}
	err = rc.ValidateClient(client, Fly.Target, false)
	if err != nil {
		return err
	}

	volumes, err := client.ListVolumes()
	if err != nil {
		return err
	}

	table := ui.Table{
		Headers: ui.TableRow{
			{Contents: "handle", Color: color.New(color.Bold)},
			{Contents: "ttl", Color: color.New(color.Bold)},
			{Contents: "validity", Color: color.New(color.Bold)},
			{Contents: "worker", Color: color.New(color.Bold)},
			{Contents: "type", Color: color.New(color.Bold)},
			{Contents: "identifier", Color: color.New(color.Bold)},
			{Contents: "size", Color: color.New(color.Bold)},
		},
	}

	sort.Sort(volumesByWorkerAndHandle(volumes))

	for _, c := range volumes {
		row := ui.TableRow{
			{Contents: c.ID},
			{Contents: formatTTL(c.TTLInSeconds)},
			{Contents: formatTTL(c.ValidityInSeconds)},
			{Contents: c.WorkerName},
			{Contents: c.Type},
			{Contents: c.Identifier},
			{Contents: fmt.Sprintf("%.1fM", float64(c.Size)/float64(1024))},
		}

		table.Data = append(table.Data, row)
	}

	return table.Render(os.Stdout)
}

type volumesByWorkerAndHandle []atc.Volume

func (cs volumesByWorkerAndHandle) Len() int          { return len(cs) }
func (cs volumesByWorkerAndHandle) Swap(i int, j int) { cs[i], cs[j] = cs[j], cs[i] }
func (cs volumesByWorkerAndHandle) Less(i int, j int) bool {
	if cs[i].WorkerName == cs[j].WorkerName {
		return cs[i].ID < cs[j].ID
	}

	return cs[i].WorkerName < cs[j].WorkerName
}

func formatTTL(ttlInSeconds int64) string {
	if ttlInSeconds == 0 {
		return "indefinite"
	}

	duration := time.Duration(ttlInSeconds) * time.Second

	return fmt.Sprintf(
		"%0.2d:%0.2d:%0.2d",
		int64(duration.Hours()),
		int64(duration.Minutes())%60,
		ttlInSeconds%60,
	)
}

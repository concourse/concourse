package commands

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/rc"
	"github.com/fatih/color"
)

type VolumesCommand struct{}

var volumesCommand VolumesCommand

func init() {
	volumes, err := Parser.AddCommand(
		"volumes",
		"Print the volumes present across your workers",
		"",
		&volumesCommand,
	)
	if err != nil {
		panic(err)
	}

	volumes.Aliases = []string{"vs"}
}

func (command *VolumesCommand) Execute([]string) error {
	target, err := rc.SelectTarget(globalOptions.Target, globalOptions.Insecure)
	if err != nil {
		log.Fatalln(err)
	}

	client, err := atcclient.NewClient(*target)
	if err != nil {
		log.Fatalln(err)
	}
	handler := atcclient.NewAtcHandler(client)

	volumes, err := handler.ListVolumes()
	if err != nil {
		log.Fatalln(err)
	}

	headers := TableRow{
		{Contents: "handle", Color: color.New(color.Bold)},
		{Contents: "ttl", Color: color.New(color.Bold)},
		{Contents: "validity", Color: color.New(color.Bold)},
		{Contents: "worker", Color: color.New(color.Bold)},
		{Contents: "version", Color: color.New(color.Bold)},
	}

	table := Table{headers}

	sort.Sort(volumesByWorkerAndHandle(volumes))

	for _, c := range volumes {
		row := TableRow{
			{Contents: c.ID},
			{Contents: formatTTL(c.TTLInSeconds)},
			{Contents: formatTTL(c.ValidityInSeconds)},
			{Contents: c.WorkerName},
			{Contents: formatVersion(c.ResourceVersion)},
		}

		table = append(table, row)
	}

	fmt.Print(table.Render())

	return nil
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
	duration := time.Duration(ttlInSeconds) * time.Second

	return fmt.Sprintf(
		"%0.2d:%0.2d:%0.2d",
		int64(duration.Hours()),
		int64(duration.Minutes())%60,
		ttlInSeconds%60,
	)
}

func formatVersion(version atc.Version) string {
	pairs := []string{}
	for k, v := range version {
		pairs = append(pairs, fmt.Sprintf("%s: %s", k, v))
	}

	sort.Sort(sort.StringSlice(pairs))

	return strings.Join(pairs, ", ")
}

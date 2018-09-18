package commands

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v2"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/fatih/color"
)

type VolumesCommand struct {
	Details bool `short:"d" long:"details" description:"Print additional information for each volume"`
	Json    bool `long:"json" description:"Print command result as JSON"`
}

func (command *VolumesCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	volumes, err := target.Team().ListVolumes()
	if err != nil {
		return err
	}

	if command.Json {
		err = displayhelpers.JsonPrint(volumes)
		if err != nil {
			return err
		}
		return nil
	}

	table := ui.Table{
		Headers: ui.TableRow{
			{Contents: "handle", Color: color.New(color.Bold)},
			{Contents: "worker", Color: color.New(color.Bold)},
			{Contents: "type", Color: color.New(color.Bold)},
			{Contents: "identifier", Color: color.New(color.Bold)},
		},
	}

	sort.Sort(volumesByWorkerAndHandle(volumes))

	for _, c := range volumes {
		row := ui.TableRow{
			{Contents: c.ID},
			{Contents: c.WorkerName},
			{Contents: c.Type},
			{Contents: command.volumeIdentifier(c)},
		}

		table.Data = append(table.Data, row)
	}

	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

func (command *VolumesCommand) volumeIdentifier(volume atc.Volume) string {
	switch volume.Type {
	case "container":
		if command.Details {
			identifier := fmt.Sprintf("container:%s,path:%s", volume.ContainerHandle, volume.Path)
			if volume.ParentHandle != "" {
				identifier = fmt.Sprintf("%s,parent:%s", identifier, volume.ParentHandle)
			}
			return identifier
		}

		return volume.ContainerHandle
	case "task-cache":
		return fmt.Sprintf("%s/%s/%s", volume.PipelineName, volume.JobName, volume.StepName)
	case "resource":
		if command.Details {
			return presentResourceType(volume.ResourceType)
		}
		return presentMap(volume.ResourceType.Version)
	case "resource-type":
		if command.Details {
			return presentMap(volume.BaseResourceType)
		}
		return volume.BaseResourceType.Name
	}

	return "n/a"
}

func presentMap(version interface{}) string {
	marshalled, _ := yaml.Marshal(version)
	lines := strings.Split(strings.TrimSpace(string(marshalled)), "\n")
	return strings.Replace(strings.Join(lines, ","), " ", "", -1)
}

func presentResourceType(resourceType *atc.VolumeResourceType) string {
	if resourceType.BaseResourceType != nil {
		return presentMap(resourceType.BaseResourceType)
	}

	if resourceType.ResourceType != nil {
		innerResourceType := presentResourceType(resourceType.ResourceType)
		version := presentMap(resourceType.Version)
		return fmt.Sprintf("type:resource(%s),version:%s", innerResourceType, version)
	}

	return ""
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

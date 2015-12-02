package commands

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/concourse/go-concourse/concourse"
	"github.com/fatih/color"
)

const timeDateLayout = "2006-1-2@15:04:05"

type BuildsCommand struct {
	Count int                 `short:"c" long:"count" default:"50"															description:"number of builds you want to limit the return to"`
	Job   flaghelpers.JobFlag `short:"j" long:"job"									value-name:"PIPELINE/JOB"		description:"Name of a job to get builds for"`
}

func (command *BuildsCommand) Execute([]string) error {
	connection, err := rc.TargetConnection(Fly.Target)
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	client := concourse.NewClient(connection)

	var builds []atc.Build
	if command.Job.PipelineName != "" && command.Job.JobName != "" {
		var found bool
		builds, _, found, err = client.JobBuilds(
			command.Job.PipelineName,
			command.Job.JobName,
			concourse.Page{Limit: command.Count},
		)
		if err != nil {
			log.Fatalln(err)
		}

		if !found {
			log.Fatalln("pipleline/job not found")
		}
	} else {
		builds, err = client.AllBuilds()
		if err != nil {
			log.Fatalln(err)
		}
	}

	table := ui.Table{
		Headers: ui.TableRow{
			{Contents: "id", Color: color.New(color.Bold)},
			{Contents: "pipeline/job#build", Color: color.New(color.Bold)},
			{Contents: "status", Color: color.New(color.Bold)},
			{Contents: "start-UTC", Color: color.New(color.Bold)},
			{Contents: "end-UTC", Color: color.New(color.Bold)},
			{Contents: "duration", Color: color.New(color.Bold)},
		},
	}

	var rangeUntil int
	if command.Count < len(builds) {
		rangeUntil = command.Count
	} else {
		rangeUntil = len(builds)
	}

	for _, b := range builds[:rangeUntil] {
		var durationCell ui.TableCell
		var startTimeCell ui.TableCell
		var endTimeCell ui.TableCell

		startTime := time.Unix(b.StartTime, 0).UTC()
		endTime := time.Unix(b.EndTime, 0).UTC()

		if b.StartTime == 0 {
			startTimeCell.Contents = "n/a"
		} else {
			startTimeCell.Contents = startTime.Format(timeDateLayout)
		}

		if b.EndTime == 0 {
			endTimeCell.Contents = "n/a"
			durationCell.Contents = fmt.Sprintf("%v+", roundSecondsOffDuration(time.Since(startTime)))
		} else {
			endTimeCell.Contents = endTime.Format(timeDateLayout)
			durationCell.Contents = endTime.Sub(startTime).String()
		}

		if b.EndTime == 0 && b.StartTime == 0 {
			durationCell.Contents = "n/a"
		}

		var pipelineJobBuildCell ui.TableCell
		if b.PipelineName == "" {
			pipelineJobBuildCell.Contents = "n/a"
		} else {
			pipelineJobBuildCell.Contents = fmt.Sprintf("%s/%s#%s", b.PipelineName, b.JobName, b.Name)
		}

		var statusCell ui.TableCell
		statusCell.Contents = b.Status

		switch b.Status {
		case "pending":
			statusCell.Color = color.New(color.FgWhite)
		case "started":
			statusCell.Color = color.New(color.FgYellow)
		case "succeeded":
			statusCell.Color = color.New(color.FgGreen)
		case "failed":
			statusCell.Color = color.New(color.FgRed)
		case "errored":
			statusCell.Color = color.New(color.FgMagenta)
		case "aborted":
			statusCell.Color = color.New(color.Underline)
		case "paused":
			statusCell.Color = color.New(color.FgCyan)
		}

		table.Data = append(table.Data, []ui.TableCell{
			{Contents: strconv.Itoa(b.ID)},
			pipelineJobBuildCell,
			statusCell,
			startTimeCell,
			endTimeCell,
			durationCell,
		})
	}

	return table.Render(os.Stdout)
}

func roundSecondsOffDuration(d time.Duration) time.Duration {
	return d - (d % time.Second)
}

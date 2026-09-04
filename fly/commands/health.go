package commands

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/concourse/concourse/v8/atc"
	"github.com/concourse/concourse/v8/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/v8/fly/rc"
	"github.com/concourse/concourse/v8/fly/ui"
	"github.com/fatih/color"
)

type HealthCommand struct {
	Json bool `long:"json" description:"Print command result as JSON"`
}

func (command *HealthCommand) Execute([]string) error {
	target, err := rc.LoadTargetWithoutAuth(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	health, err := target.Client().GetHealth()
	if err != nil {
		return err
	}

	if command.Json {
		return displayhelpers.JsonPrint(health)
	}

	return command.renderTable(health)
}

func (command *HealthCommand) renderTable(health atc.Health) error {
	headers := ui.TableRow{
		{Contents: "subsystem", Color: color.New(color.Bold)},
		{Contents: "status", Color: color.New(color.Bold)},
		{Contents: "detail", Color: color.New(color.Bold)},
	}

	table := ui.Table{Headers: headers}

	table.Data = append(table.Data, ui.TableRow{
		{Contents: "overall"},
		statusCell(string(health.Status)),
		{Contents: health.Timestamp.UTC().Format(time.RFC3339)},
	})

	table.Data = append(table.Data, ui.TableRow{
		{Contents: "database"},
		statusCell(string(health.Database.Status)),
		{Contents: health.Database.Error},
	})

	workerDetail := fmt.Sprintf("%d/%d running", health.Workers.Running, health.Workers.Total)
	if len(health.Workers.UnhealthyWorkers) > 0 {
		workerDetail += fmt.Sprintf(" (unhealthy: %s)", strings.Join(health.Workers.UnhealthyWorkers, ", "))
	}
	table.Data = append(table.Data, ui.TableRow{
		{Contents: "workers"},
		statusCell(health.Workers.Status),
		{Contents: workerDetail},
	})

	for _, c := range health.Components {
		var details []string

		if c.Paused {
			details = append(details, "paused")
		}

		if !c.LastRan.IsZero() {
			details = append(details, "last ran: "+c.LastRan.UTC().Format(time.RFC3339))
		}

		table.Data = append(table.Data, ui.TableRow{
			{Contents: c.Name},
			statusCell(string(c.Status)),
			{Contents: strings.Join(details, ", ")},
		})
	}

	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

func statusCell(status string) ui.TableCell {
	switch status {
	case string(atc.HealthStatusOK), string(atc.HealthStatusHealthy):
		return ui.TableCell{Contents: status, Color: ui.SucceededColor}
	case string(atc.HealthStatusDegraded):
		return ui.TableCell{Contents: status, Color: ui.StartedColor}
	case string(atc.HealthStatusFailing), string(atc.HealthStatusUnhealthy):
		return ui.TableCell{Contents: status, Color: ui.FailedColor}
	default:
		return ui.TableCell{Contents: status, Color: ui.OffColor}
	}
}

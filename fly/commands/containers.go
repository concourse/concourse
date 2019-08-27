package commands

import (
	"errors"
	"os"
	"sort"
	"strconv"

	"github.com/concourse/concourse/go-concourse/concourse"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
)

type ContainersCommand struct {
	Json bool `long:"json" description:"Print command result as JSON"`
	TeamsParam
}

func (command *ContainersCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	if len(command.TeamsFlags) > 0 && command.AllTeams {
		return errors.New("cannot specify both --all-teams and --team")
	}

	var containers []atc.Container
	client := target.Client()
	if command.AllTeams {
		containers, err = client.ListAllContainers()
		if err != nil {
			return err
		}
	} else {
		var teams []concourse.Team
		if len(command.TeamsFlags) > 0 {
			for _, teamName := range command.TeamsFlags {
				teams = append(teams, client.Team(teamName))
			}
		} else {
			teams = append(teams, target.Team())
		}
		for _, team := range teams {
			teamContainers, err := team.ListContainers(map[string]string{})
			if err != nil {
				return err
			}
			containers = append(containers, teamContainers...)
		}
	}

	if command.Json {
		err = displayhelpers.JsonPrint(containers)
		if err != nil {
			return err
		}
		return nil
	}

	table := ui.Table{
		Headers: ui.TableRow{
			{Contents: "handle", Color: color.New(color.Bold)},
			{Contents: "worker", Color: color.New(color.Bold)},
			{Contents: "pipeline", Color: color.New(color.Bold)},
			{Contents: "job", Color: color.New(color.Bold)},
			{Contents: "build #", Color: color.New(color.Bold)},
			{Contents: "build id", Color: color.New(color.Bold)},
			{Contents: "type", Color: color.New(color.Bold)},
			{Contents: "name", Color: color.New(color.Bold)},
			{Contents: "attempt", Color: color.New(color.Bold)},
			{Contents: "team", Color: color.New(color.Bold)},
		},
	}

	for _, c := range containers {
		row := ui.TableRow{
			{Contents: c.ID},
			{Contents: c.WorkerName},
			stringOrDefault(c.PipelineName),
			stringOrDefault(c.JobName),
			stringOrDefault(c.BuildName),
			buildIDOrNone(c.BuildID),
			{Contents: c.Type},
			stringOrDefault(c.StepName + c.ResourceName),
			stringOrDefault(c.Attempt, "n/a"),
			stringOrDefault(c.TeamName, "n/a"),
		}

		table.Data = append(table.Data, row)
	}

	sort.Sort(table.Data)

	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

func buildIDOrNone(id int) ui.TableCell {
	var column ui.TableCell

	if id == 0 {
		column.Contents = "none"
		column.Color = ui.OffColor
	} else {
		column.Contents = strconv.Itoa(id)
	}

	return column
}

func stringOrDefault(containerType string, def ...string) ui.TableCell {
	var column ui.TableCell

	column.Contents = containerType
	if column.Contents == "" || column.Contents == "[]" {
		if len(def) == 0 {
			column.Contents = "none"
			column.Color = color.New(color.Faint)
		} else {
			column.Contents = def[0]
		}
	}

	return column
}

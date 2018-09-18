package commands

import (
	"os"
	"sort"

	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/fatih/color"
	"strings"
)

type TeamsCommand struct {
	Json    bool `long:"json" description:"Print command result as JSON"`
	Details bool `short:"d" long:"details" description:"Print authentication configuration"`
}

func (command *TeamsCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	teams, err := target.Client().ListTeams()
	if err != nil {
		return err
	}

	if command.Json {
		err = displayhelpers.JsonPrint(teams)
		if err != nil {
			return err
		}
		return nil
	}

	headers := ui.TableRow{
		{Contents: "name", Color: color.New(color.Bold)},
	}

	if command.Details {
		headers = append(headers,
			ui.TableCell{Contents: "users", Color: color.New(color.Bold)},
			ui.TableCell{Contents: "groups", Color: color.New(color.Bold)},
		)
	}

	table := ui.Table{Headers: headers}

	for _, t := range teams {
		row := ui.TableRow{
			{Contents: t.Name},
		}

		if command.Details {
			var usersCell, groupsCell ui.TableCell

			hasUsers := len(t.Auth["users"]) != 0
			hasGroups := len(t.Auth["groups"]) != 0

			if !hasUsers && !hasGroups {
				usersCell.Contents = "all"
				usersCell.Color = color.New(color.Faint)
			} else if !hasUsers {
				usersCell.Contents = "none"
				usersCell.Color = color.New(color.Faint)
			} else {
				usersCell.Contents = strings.Join(t.Auth["users"], ",")
			}

			if hasGroups {
				groupsCell.Contents = strings.Join(t.Auth["groups"], ",")
			} else {
				groupsCell.Contents = "none"
				groupsCell.Color = color.New(color.Faint)
			}

			row = append(row, usersCell)
			row = append(row, groupsCell)
		}

		table.Data = append(table.Data, row)
	}

	sort.Sort(table.Data)

	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

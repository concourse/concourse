package commands

import (
	"fmt"
	"os"
	"sort"

	"strings"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
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

	var headers ui.TableRow
	if command.Details {
		headers = ui.TableRow{
			{Contents: "name/role", Color: color.New(color.Bold)},
			{Contents: "users", Color: color.New(color.Bold)},
			{Contents: "groups", Color: color.New(color.Bold)},
		}
	} else {
		headers = ui.TableRow{
			{Contents: "name", Color: color.New(color.Bold)},
		}
	}

	table := ui.Table{Headers: headers}

	for _, t := range teams {

		if command.Details {
			for role, auth := range t.Auth {
				row := ui.TableRow{
					{Contents: fmt.Sprintf("%s/%s", t.Name, role)},
				}
				var usersCell, groupsCell ui.TableCell

				hasUsers := len(auth["users"]) != 0
				hasGroups := len(auth["groups"]) != 0

				if !hasUsers && !hasGroups {
					usersCell.Contents = "all"
					usersCell.Color = color.New(color.Faint)
				} else if !hasUsers {
					usersCell.Contents = "none"
					usersCell.Color = color.New(color.Faint)
				} else {
					usersCell.Contents = strings.Join(auth["users"], ",")
				}

				if hasGroups {
					groupsCell.Contents = strings.Join(auth["groups"], ",")
				} else {
					groupsCell.Contents = "none"
					groupsCell.Color = color.New(color.Faint)
				}

				row = append(row, usersCell)
				row = append(row, groupsCell)
				table.Data = append(table.Data, row)
			}

		} else {
			row := ui.TableRow{
				{Contents: t.Name},
			}
			table.Data = append(table.Data, row)
		}
	}

	sort.Sort(table.Data)

	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

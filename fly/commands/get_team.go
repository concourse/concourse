package commands

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/concourse/concourse/v5/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/v5/fly/rc"
	"github.com/concourse/concourse/v5/fly/ui"
	"github.com/fatih/color"
)

type GetTeamCommand struct {
	Team string `short:"n" long:"team" required:"true" description:"Get configuration of this team"`
	Json bool   `short:"j" long:"json" description:"Print config as json instead of yaml"`
}

func (command *GetTeamCommand) Execute(args []string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	if err := target.Validate(); err != nil {
		return err
	}

	team, found, err := target.Team().Team(command.Team)
	if err != nil {
		return err
	}

	if !found {
		return errors.New("team not found")
	}

	if command.Json {
		if err := displayhelpers.JsonPrint(team); err != nil {
			return err
		}
		return nil
	}

	headers := ui.TableRow{
		{Contents: "name/role", Color: color.New(color.Bold)},
		{Contents: "users", Color: color.New(color.Bold)},
		{Contents: "groups", Color: color.New(color.Bold)},
	}
	table := ui.Table{Headers: headers}
	for role, auth := range team.Auth {
		row := ui.TableRow{
			{Contents: fmt.Sprintf("%s/%s", team.Name, role)},
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
	sort.Sort(table.Data)
	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

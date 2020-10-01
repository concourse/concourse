package commands

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
)

type GetTeamCommand struct {
	Team flaghelpers.TeamFlag `short:"n" long:"team-name" required:"true" description:"Get configuration of this team"`
	JSON bool                 `short:"j" long:"json" description:"Print command result as JSON"`
}

func (command *GetTeamCommand) Execute(args []string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	if err := target.Validate(); err != nil {
		return err
	}

	teamName := command.Team.Name()
	team, err := target.FindTeam(teamName)
	if err != nil {
		return err
	}

	if command.JSON {
		err := displayhelpers.JsonPrint(team.ATCTeam())
		if err != nil {
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
	for role, auth := range team.Auth() {
		row := ui.TableRow{
			{Contents: fmt.Sprintf("%s/%s", team.Name(), role)},
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

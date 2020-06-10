package commands

import (
	"os"
	"sort"
	"strings"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
)

type UserinfoCommand struct {
	Json bool `long:"json" description:"Print command result as JSON"`
}

func (command *UserinfoCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	userinfo, err := target.Client().UserInfo()
	if err != nil {
		return err
	}

	if command.Json {
		err = displayhelpers.JsonPrint(userinfo)
		if err != nil {
			return err
		}
		return nil
	}

	headers := ui.TableRow{
		{Contents: "username", Color: color.New(color.Bold)},
		{Contents: "team/role", Color: color.New(color.Bold)},
	}

	table := ui.Table{Headers: headers}

	var teamRoles []string
	for team, roles := range userinfo.Teams {
		for _, role := range roles {
			teamRoles = append(teamRoles, team+"/"+role)
		}
	}

	sort.Strings(teamRoles)

	row := ui.TableRow{
		{Contents: userinfo.UserName},
		{Contents: strings.Join(teamRoles, ",")},
	}

	table.Data = append(table.Data, row)

	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

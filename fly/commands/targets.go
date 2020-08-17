package commands

import (
	"os"
	"sort"
	"time"

	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/concourse/concourse/skymarshal/token"
	"github.com/fatih/color"
)

type TargetsCommand struct{}

func (command *TargetsCommand) Execute([]string) error {
	targets, err := rc.LoadTargets()
	if err != nil {
		return err
	}

	table := ui.Table{
		Headers: ui.TableRow{
			{Contents: "name", Color: color.New(color.Bold)},
			{Contents: "url", Color: color.New(color.Bold)},
			{Contents: "team", Color: color.New(color.Bold)},
			{Contents: "expiry", Color: color.New(color.Bold)},
		},
	}

	for targetName, targetValues := range targets {
		expirationTime := getExpirationFromString(targetValues.Token)

		row := ui.TableRow{
			{Contents: string(targetName)},
			{Contents: targetValues.API},
			{Contents: targetValues.TeamName},
			{Contents: expirationTime},
		}

		table.Data = append(table.Data, row)
	}

	sort.Sort(table.Data)

	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

func getExpirationFromString(ttoken *rc.TargetToken) string {
	if ttoken == nil || ttoken.Type == "" || ttoken.Value == "" {
		return "n/a"
	}

	expiry, err := token.Factory{}.ParseExpiry(ttoken.Value)
	if err != nil {
		return "n/a: invalid token"
	}

	return expiry.UTC().Format(time.RFC1123)
}

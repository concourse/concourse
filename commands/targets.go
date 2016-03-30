package commands

import (
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/dgrijalva/jwt-go"
	"github.com/fatih/color"
)

type TargetsCommand struct{}

func (command *TargetsCommand) Execute([]string) error {
	flyYAML, err := rc.LoadTargets()
	if err != nil {
		return err
	}

	table := ui.Table{
		Headers: ui.TableRow{
			{Contents: "name", Color: color.New(color.Bold)},
			{Contents: "url", Color: color.New(color.Bold)},
			{Contents: "expiry", Color: color.New(color.Bold)},
		},
	}

	for targetName, targetValues := range flyYAML.Targets {
		expirationTime, err := GetExpirationFromString(targetValues.Token.Value)
		if err != nil {
			return err
		}

		row := ui.TableRow{
			{Contents: string(targetName)},
			{Contents: targetValues.API},
			{Contents: expirationTime},
		}

		table.Data = append(table.Data, row)
	}

	sort.Sort(table.Data)

	return table.Render(os.Stdout)
}

func GetExpirationFromString(token string) (string, error) {
	parsedToken, _ := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return "", nil
	})

	var (
		ok       bool
		expClaim interface{}
	)
	if expClaim, ok = parsedToken.Claims["exp"]; !ok {
		return "", nil
	}

	intSeconds, err := strconv.ParseInt(string(expClaim.(string)), 10, 64)
	if err != nil {
		return "", nil
	}

	unixSeconds := time.Unix(intSeconds, 0)

	return unixSeconds.Format(time.RFC1123), nil
}

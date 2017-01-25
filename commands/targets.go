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
			{Contents: "team", Color: color.New(color.Bold)},
			{Contents: "expiry", Color: color.New(color.Bold)},
		},
	}

	for targetName, targetValues := range flyYAML.Targets {
		expirationTime := GetExpirationFromString(targetValues.Token)

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

func GetExpirationFromString(token *rc.TargetToken) string {
	if token == nil || token.Type == "" || token.Value == "" {
		return "n/a"
	}

	parsedToken, _ := jwt.Parse(token.Value, func(token *jwt.Token) (interface{}, error) {
		return "", nil
	})

	claims := parsedToken.Claims.(jwt.MapClaims)
	expClaim, ok := claims["exp"]
	if !ok {
		return "n/a"
	}

	var intSeconds int64

	floatSeconds, ok := expClaim.(float64)
	if ok {
		intSeconds = int64(floatSeconds)
	} else {
		stringSeconds, ok := expClaim.(string)
		if !ok {
			return "n/a"
		}
		var err error
		intSeconds, err = strconv.ParseInt(stringSeconds, 10, 64)
		if err != nil {
			return "n/a"
		}
	}

	unixSeconds := time.Unix(intSeconds, 0).UTC()

	return unixSeconds.Format(time.RFC1123)
}

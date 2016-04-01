package commands

import (
	"log"
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

	if len(flyYAML.Targets) == 0 {
		log.Fatalln("no targets found, please add some and try again")
	}

	table := ui.Table{
		Headers: ui.TableRow{
			{Contents: "name", Color: color.New(color.Bold)},
			{Contents: "url", Color: color.New(color.Bold)},
			{Contents: "expiry", Color: color.New(color.Bold)},
		},
	}

	for targetName, targetValues := range flyYAML.Targets {
		expirationTime := GetExpirationFromString(targetValues.Token)

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

func GetExpirationFromString(token *rc.TargetToken) string {
	if token == nil {
		return "n/a"
	}

	parsedToken, _ := jwt.Parse(token.Value, func(token *jwt.Token) (interface{}, error) {
		return "", nil
	})

	var (
		ok       bool
		expClaim interface{}
	)
	if expClaim, ok = parsedToken.Claims["exp"]; !ok {
		return "n/a"
	}

	intSeconds, err := strconv.ParseInt(string(expClaim.(string)), 10, 64)
	if err != nil {
		return "n/a"
	}

	unixSeconds := time.Unix(intSeconds, 0)

	return unixSeconds.Format(time.RFC1123)
}

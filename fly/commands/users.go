package commands

import (
	"errors"
	"os"
	"time"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
)

const inputDateLayout = "2006-01-02"

type ActiveUsersCommand struct {
	Since string `long:"since" description:"Start date range of returned users' last login, defaults to 2 months from today'"`
	Json  bool   `long:"json" description:"Print command result as JSON"`
}

func (command *ActiveUsersCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	dateSince := time.Now().AddDate(0, -2, 0)

	if command.Since != "" {
		dateSince, err = time.ParseInLocation(inputDateLayout, command.Since, time.Now().Location())
		if err != nil {
			return errors.New("since time should be in the format: yyyy-mm-dd")
		}
	}

	if dateSince.After(time.Now()) {
		return errors.New("since time can't be in the future")
	}

	users, err := target.Client().ListActiveUsersSince(dateSince)
	if err != nil {
		return err
	}

	if command.Json {
		err = displayhelpers.JsonPrint(users)
		if err != nil {
			return err
		}
		return nil
	}

	headers := ui.TableRow{
		{Contents: "username", Color: color.New(color.Bold)},
		{Contents: "connector", Color: color.New(color.Bold)},
		{Contents: "last login", Color: color.New(color.Bold)},
	}

	table := ui.Table{Headers: headers}

	for _, user := range users {
		row := ui.TableRow{
			{Contents: user.Username},
			{Contents: user.Connector},
			{Contents: time.Unix(user.LastLogin, 0).Format(inputDateLayout)},
		}
		table.Data = append(table.Data, row)
	}
	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

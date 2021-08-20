package commands

import (
	"errors"
	"fmt"

	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/vito/go-interact/interact"
)

type DestroyWebhookCommand struct {
	SkipInteractive bool   `short:"n"  long:"non-interactive" description:"Skips confirmation interation"`
	Name            string `short:"w"  long:"webhook"  required:"true"  description:"Webhook to configure"`
	Team            string `           long:"team"     required:"false" description:"Name of the team to which the webhook should belong, if different from the target default"`
}

func (command *DestroyWebhookCommand) Execute(args []string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	var team concourse.Team
	if command.Team != "" {
		team, err = target.FindTeam(command.Team)
		if err != nil {
			return err
		}
	} else {
		team = target.Team()
	}

	fmt.Printf("!!! this will remove all data for webbook `%s`\n\n", command.Name)
	if !command.SkipInteractive {
		var confirm string
		err := interact.NewInteraction("please type the webhook name to confirm").Resolve(interact.Required(&confirm))
		if err != nil {
		}
		if confirm != command.Name {
			return errors.New("incorrect webhook name; bailing out")
		}
	}

	err = team.DestroyWebhook(command.Name)
	if err != nil {
		return err
	}

	fmt.Println("webhook destroyed")
	return nil
}

package commands

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/vito/go-interact/interact"
)

type SetWebhookCommand struct {
	SkipInteractive bool `short:"n"  long:"non-interactive" description:"Skips confirmation interation"`

	Name  string `short:"w"  long:"webhook"  required:"true"  description:"Webhook to configure"`
	Type  string `           long:"type"     required:"true"  description:"The type of webhook to configure (e.g. github, bitbucket, etc.)"`
	Token string `           long:"token"    required:"false" description:"Token added as a query parameter in the webhook endpoint. If unset, one will be randomly generated."`

	Team string `            long:"team"     required:"false" description:"Name of the team to which the webhook should belong, if different from the target default"`
}

func (command *SetWebhookCommand) Execute(args []string) error {
	webhook := atc.Webhook{
		Name: command.Name,
		Type: command.Type,
	}
	if command.Token != "" {
		webhook.Token = command.Token
	} else {
		var randomTokenBytes [16]byte
		if _, err := rand.Read(randomTokenBytes[:]); err != nil {
			return err
		}
		webhook.Token = base64.RawURLEncoding.EncodeToString(randomTokenBytes[:])
	}

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

	fmt.Println("\n\x1b[1mwarning\x1b[m: if the webhook already exists, this operation will overwrite the type and token")

	if !command.SkipInteractive {
		var confirm bool
		err := interact.NewInteraction("apply webhook configuration?").Resolve(&confirm)
		if err != nil {
		}
		if !confirm {
			displayhelpers.Failf("bailing out")
		}
	}

	created, err := team.SetWebhook(webhook)
	if err != nil {
		return err
	}

	if created {
		fmt.Println("webhook created")
	} else {
		fmt.Println("webhook updated")
	}

	fmt.Println("\nconfigure your external service with the following webhook URL:")
	fmt.Printf("\n%s/api/v1/teams/%s/webhooks/%s?token=%s\n\n", target.URL(), team.Name(), webhook.Name, webhook.Token)

	return nil
}

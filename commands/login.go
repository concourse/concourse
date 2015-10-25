package commands

import (
	"fmt"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/rc"
	"github.com/vito/go-interact/interact"
)

type LoginCommand struct {
	ATCURL   string `short:"c" long:"concourse-url" description:"Concourse URL to authenticate with"`
	Insecure bool   `short:"k" long:"insecure" description:"Skip verification of the endpoint's SSL certificate"`
}

var loginCommand LoginCommand

func init() {
	_, err := Parser.AddCommand(
		"login",
		"Authenticate with the target",
		"",
		&loginCommand,
	)
	if err != nil {
		panic(err)
	}
}

func (command *LoginCommand) Execute(args []string) error {
	client, err := rc.NewClient(command.ATCURL, command.Insecure)
	if err != nil {
		return err
	}

	handler := atcclient.NewAtcHandler(client)

	authMethods, err := handler.ListAuthMethods()
	if err != nil {
		return err
	}

	choices := make([]interact.Choice, len(authMethods))
	for i, method := range authMethods {
		choices[i] = interact.Choice{
			Display: method.DisplayName,
			Value:   method,
		}
	}

	var chosenMethod atc.AuthMethod
	err = interact.NewInteraction("choose an auth method", choices...).Resolve(&chosenMethod)
	if err != nil {
		return err
	}

	return command.loginWith(chosenMethod, client)
}

func (command *LoginCommand) loginWith(method atc.AuthMethod, client atcclient.Client) error {
	switch method.Type {
	case atc.AuthTypeOAuth:
		fmt.Println("navigate to the following URL in your browser:")
		fmt.Println("")
		fmt.Printf("    %s\n", method.AuthURL)
		fmt.Println("")

		var token string
		err := interact.NewInteraction("enter token").Resolve(interact.Required(&token))
		if err != nil {
			return err
		}

		fmt.Println("token saved")

	case atc.AuthTypeBasic:
		var username string
		err := interact.NewInteraction("username").Resolve(interact.Required(&username))
		if err != nil {
			return err
		}

		var password interact.Password
		err = interact.NewInteraction("password").Resolve(interact.Required(&password))
		if err != nil {
			return err
		}

		basicAuthClient, err := atcclient.NewClient(
			client.URL(),
			&http.Client{
				Transport: basicAuthTransport{
					username: username,
					password: string(password),
					base:     client.HTTPClient().Transport,
				},
			},
		)
		if err != nil {
			return err
		}

		handler := atcclient.NewAtcHandler(basicAuthClient)

		token, err := handler.AuthToken()
		if err != nil {
			return err
		}

		err = rc.SaveTarget(
			globalOptions.Target,
			command.ATCURL,
			command.Insecure,
			&rc.TargetToken{
				Type:  token.Type,
				Value: token.Value,
			},
		)
		if err != nil {
			return err
		}

		fmt.Println("token saved")
	}

	return nil
}

type basicAuthTransport struct {
	username string
	password string

	base http.RoundTripper
}

func (t basicAuthTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.SetBasicAuth(t.username, t.password)
	return t.base.RoundTrip(r)
}

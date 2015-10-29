package commands

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/concourse/atc"
	"github.com/concourse/fly/rc"
	"github.com/concourse/go-concourse/concourse"
	"github.com/vito/go-interact/interact"
)

type LoginCommand struct {
	ATCURL   string `short:"c" long:"concourse-url" description:"Concourse URL to authenticate with"`
	Insecure bool   `short:"k" long:"insecure" description:"Skip verification of the endpoint's SSL certificate"`
}

func (command *LoginCommand) Execute(args []string) error {
	var connection concourse.Connection
	var err error

	if command.ATCURL != "" {
		connection, err = rc.NewConnection(command.ATCURL, command.Insecure)
	} else {
		connection, err = rc.TargetConnection(Fly.Target)
	}

	if err != nil {
		return err
	}

	client := concourse.NewClient(connection)

	authMethods, err := client.ListAuthMethods()
	if err != nil {
		return err
	}

	var chosenMethod atc.AuthMethod
	switch len(authMethods) {
	case 0:
		fmt.Println("no auth methods configured; updating target data")
		err := rc.SaveTarget(
			Fly.Target,
			connection.URL(),
			command.Insecure,
			&rc.TargetToken{},
		)

		if err != nil {
			return err
		}
		return nil
	case 1:
		chosenMethod = authMethods[0]
	default:
		choices := make([]interact.Choice, len(authMethods))
		for i, method := range authMethods {
			choices[i] = interact.Choice{
				Display: method.DisplayName,
				Value:   method,
			}
		}

		err = interact.NewInteraction("choose an auth method", choices...).Resolve(&chosenMethod)
		if err != nil {
			return err
		}
	}

	return command.loginWith(chosenMethod, connection)
}

func (command *LoginCommand) loginWith(method atc.AuthMethod, connection concourse.Connection) error {
	var token atc.AuthToken

	switch method.Type {
	case atc.AuthTypeOAuth:
		fmt.Println("navigate to the following URL in your browser:")
		fmt.Println("")
		fmt.Printf("    %s\n", method.AuthURL)
		fmt.Println("")

		for {
			var tokenStr string

			err := interact.NewInteraction("enter token").Resolve(interact.Required(&tokenStr))
			if err != nil {
				return err
			}

			segments := strings.SplitN(tokenStr, " ", 2)
			if len(segments) != 2 {
				fmt.Println("token must be of the format 'TYPE VALUE', e.g. 'Bearer ...'")
				continue
			}

			token.Type = segments[0]
			token.Value = segments[1]

			break
		}

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

		newUnauthedClient, err := rc.NewConnection(connection.URL(), command.Insecure)
		if err != nil {
			return err
		}

		basicAuthClient, err := concourse.NewConnection(
			newUnauthedClient.URL(),
			&http.Client{
				Transport: basicAuthTransport{
					username: username,
					password: string(password),
					base:     newUnauthedClient.HTTPClient().Transport,
				},
			},
		)
		if err != nil {
			return err
		}

		client := concourse.NewClient(basicAuthClient)

		token, err = client.AuthToken()
		if err != nil {
			return err
		}
	}

	err := rc.SaveTarget(
		Fly.Target,
		connection.URL(),
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

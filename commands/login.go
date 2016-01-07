package commands

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/concourse/atc"
	"github.com/concourse/fly/rc"
	"github.com/concourse/go-concourse/concourse"
	"github.com/vito/go-interact/interact"
)

type LoginCommand struct {
	ATCURL   string `short:"c" long:"concourse-url" description:"Concourse URL to authenticate with"`
	Insecure bool   `short:"k" long:"insecure" description:"Skip verification of the endpoint's SSL certificate"`
	Username string `short:"u" long:"username" description:"Username for basic auth"`
	Password string `short:"p" long:"password" description:"Password for basic auth"`
}

func (command *LoginCommand) Execute(args []string) error {
	if isURL(Fly.Target) {
		return errors.New("name for the target must be specified (--target/-t)")
	}

	var connection concourse.Connection
	var err error

	if command.ATCURL != "" {
		connection, err = rc.NewConnection(command.ATCURL, command.Insecure)
	} else {
		connection, err = rc.CommandTargetConnection(Fly.Target, &command.Insecure)
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
	if command.Username != "" && command.Password != "" {
		for _, method := range authMethods {
			if method.Type == atc.AuthTypeBasic {
				chosenMethod = method
				break
			}
		}

		if chosenMethod.Type == "" {
			return errors.New("basic auth is not available")
		}
	} else {
		switch len(authMethods) {
		case 0:
			return command.saveTarget(
				connection.URL(),
				&rc.TargetToken{},
			)
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
		if command.Username != "" {
			username = command.Username
		} else {
			err := interact.NewInteraction("username").Resolve(interact.Required(&username))
			if err != nil {
				return err
			}
		}

		var password string
		if command.Password != "" {
			password = command.Password
		} else {
			var interactivePassword interact.Password
			err := interact.NewInteraction("password").Resolve(interact.Required(&interactivePassword))
			if err != nil {
				return err
			}
			password = string(interactivePassword)
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
					password: password,
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

	return command.saveTarget(
		connection.URL(),
		&rc.TargetToken{
			Type:  token.Type,
			Value: token.Value,
		},
	)
}

func (command *LoginCommand) saveTarget(url string, token *rc.TargetToken) error {
	err := rc.SaveTarget(
		Fly.Target,
		url,
		command.Insecure,
		&rc.TargetToken{
			Type:  token.Type,
			Value: token.Value,
		},
	)
	if err != nil {
		return err
	}

	fmt.Println("target saved")

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

func isURL(passedURL string) bool {
	matched, _ := regexp.MatchString("^http[s]?://", passedURL)
	return matched
}

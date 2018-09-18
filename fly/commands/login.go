package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/concourse/atc"
	"github.com/concourse/fly/rc"
	"github.com/concourse/go-concourse/concourse"
	semisemanticversion "github.com/cppforlife/go-semi-semantic/version"
	"github.com/skratchdot/open-golang/open"
	"github.com/vito/go-interact/interact"
	"golang.org/x/oauth2"
)

type LoginCommand struct {
	ATCURL      string       `short:"c" long:"concourse-url" description:"Concourse URL to authenticate with"`
	Insecure    bool         `short:"k" long:"insecure" description:"Skip verification of the endpoint's SSL certificate"`
	Username    string       `short:"u" long:"username" description:"Username for basic auth"`
	Password    string       `short:"p" long:"password" description:"Password for basic auth"`
	TeamName    string       `short:"n" long:"team-name" description:"Team to authenticate with"`
	CACert      atc.PathFlag `long:"ca-cert" description:"Path to Concourse PEM-encoded CA certificate file."`
	OpenBrowser bool         `short:"b" long:"open-browser" description:"Open browser to the auth endpoint"`
}

func (command *LoginCommand) Execute(args []string) error {
	if Fly.Target == "" {
		return errors.New("name for the target must be specified (--target/-t)")
	}

	var target rc.Target
	var err error

	var caCert string
	if command.CACert != "" {
		caCertBytes, err := ioutil.ReadFile(string(command.CACert))
		if err != nil {
			return err
		}
		caCert = string(caCertBytes)
	}

	if command.ATCURL != "" {
		if command.TeamName == "" {
			command.TeamName = atc.DefaultTeamName
		}

		target, err = rc.NewUnauthenticatedTarget(
			Fly.Target,
			command.ATCURL,
			command.TeamName,
			command.Insecure,
			caCert,
			Fly.Verbose,
		)
	} else {
		target, err = rc.LoadUnauthenticatedTarget(
			Fly.Target,
			command.TeamName,
			command.Insecure,
			caCert,
			Fly.Verbose,
		)
	}
	if err != nil {
		return err
	}

	client := target.Client()
	command.TeamName = target.Team().Name()

	fmt.Printf("logging in to team '%s'\n\n", command.TeamName)

	if len(args) != 0 {
		return errors.New("unexpected argument [" + strings.Join(args, ", ") + "]")
	}

	err = target.ValidateWithWarningOnly()
	if err != nil {
		return err
	}

	var tokenType string
	var tokenValue string

	version, err := target.Version()
	if err != nil {
		return err
	}

	semver, err := semisemanticversion.NewVersionFromString(version)
	if err != nil {
		return err
	}

	legacySemver, err := semisemanticversion.NewVersionFromString("3.14.1")
	if err != nil {
		return err
	}

	devSemver, err := semisemanticversion.NewVersionFromString("0.0.0-dev")
	if err != nil {
		return err
	}

	if semver.Compare(legacySemver) <= 0 && semver.Compare(devSemver) != 0 {
		// Legacy Auth Support
		tokenType, tokenValue, err = command.legacyAuth(target)
	} else {
		if command.Username != "" && command.Password != "" {
			tokenType, tokenValue, err = command.passwordGrant(client, command.Username, command.Password)
		} else {
			tokenType, tokenValue, err = command.authCodeGrant(client.URL())
		}
	}

	if err != nil {
		return err
	}

	fmt.Println("")

	return command.saveTarget(
		client.URL(),
		&rc.TargetToken{
			Type:  tokenType,
			Value: tokenValue,
		},
		target.CACert(),
	)
}

func (command *LoginCommand) passwordGrant(client concourse.Client, username, password string) (string, string, error) {

	oauth2Config := oauth2.Config{
		ClientID:     "fly",
		ClientSecret: "Zmx5",
		Endpoint:     oauth2.Endpoint{TokenURL: client.URL() + "/sky/token"},
		Scopes:       []string{"openid", "profile", "email", "federated:id", "groups"},
	}

	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, client.HTTPClient())

	token, err := oauth2Config.PasswordCredentialsToken(ctx, username, password)
	if err != nil {
		return "", "", err
	}

	return token.TokenType, token.AccessToken, nil
}

func (command *LoginCommand) authCodeGrant(targetUrl string) (string, string, error) {

	var tokenStr string

	stdinChannel := make(chan string)
	tokenChannel := make(chan string)
	errorChannel := make(chan error)
	portChannel := make(chan string)

	go listenForTokenCallback(tokenChannel, errorChannel, portChannel, targetUrl)

	port := <-portChannel

	redirectUri, err := url.Parse("http://127.0.0.1:" + port + "/auth/callback")
	if err != nil {
		panic(err)
	}

	openURL := fmt.Sprintf("%s/sky/login?redirect_uri=%s", targetUrl, redirectUri.String())

	fmt.Println("navigate to the following URL in your browser:")
	fmt.Println("")
	fmt.Printf("  %s\n", openURL)
	fmt.Println("")

	if command.OpenBrowser {
		// try to open the browser window, but don't get all hung up if it
		// fails, since we already printed about it.
		_ = open.Start(openURL)
	}

	go waitForTokenInput(stdinChannel, errorChannel)

	select {
	case tokenStrMsg := <-tokenChannel:
		tokenStr = tokenStrMsg
	case tokenStrMsg := <-stdinChannel:
		tokenStr = tokenStrMsg
	case errorMsg := <-errorChannel:
		return "", "", errorMsg
	}

	segments := strings.SplitN(tokenStr, " ", 2)

	return segments[0], segments[1], nil
}

func listenForTokenCallback(tokenChannel chan string, errorChannel chan error, portChannel chan string, targetUrl string) {
	s := &http.Server{
		Addr: "127.0.0.1:0",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenChannel <- r.FormValue("token")
			http.Redirect(w, r, fmt.Sprintf("%s/public/fly_success", targetUrl), http.StatusTemporaryRedirect)
		}),
	}

	err := listenAndServeWithPort(s, portChannel)

	if err != nil {
		errorChannel <- err
	}
}

func listenAndServeWithPort(srv *http.Server, portChannel chan string) error {
	addr := srv.Addr
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		return err
	}

	portChannel <- port

	return srv.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)})
}

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func waitForTokenInput(tokenChannel chan string, errorChannel chan error) {
	for {
		fmt.Printf("or enter token manually: ")

		var tokenType string
		var tokenValue string
		count, err := fmt.Scanf("%s %s", &tokenType, &tokenValue)
		if err != nil {
			if count != 2 {
				fmt.Println("token must be of the format 'TYPE VALUE', e.g. 'Bearer ...'")
				continue
			}

			errorChannel <- err
			return
		}

		tokenChannel <- tokenType + " " + tokenValue
		break
	}
}

func (command *LoginCommand) saveTarget(url string, token *rc.TargetToken, caCert string) error {
	err := rc.SaveTarget(
		Fly.Target,
		url,
		command.Insecure,
		command.TeamName,
		&rc.TargetToken{
			Type:  token.Type,
			Value: token.Value,
		},
		caCert,
	)
	if err != nil {
		return err
	}

	fmt.Println("target saved")

	return nil
}

func (command *LoginCommand) legacyAuth(target rc.Target) (string, string, error) {

	httpClient := target.Client().HTTPClient()

	authResponse, err := httpClient.Get(target.URL() + "/api/v1/teams/" + target.Team().Name() + "/auth/methods")
	if err != nil {
		return "", "", err
	}

	type authMethod struct {
		Type        string `json:"type"`
		DisplayName string `json:"display_name"`
		AuthURL     string `json:"auth_url"`
	}

	defer authResponse.Body.Close()

	var authMethods []authMethod
	json.NewDecoder(authResponse.Body).Decode(&authMethods)

	var chosenMethod authMethod

	if command.Username != "" || command.Password != "" {
		for _, method := range authMethods {
			if method.Type == "basic" {
				chosenMethod = method
				break
			}
		}

		if chosenMethod.Type == "" {
			return "", "", errors.New("basic auth is not available")
		}
	}

	choices := make([]interact.Choice, len(authMethods))

	for i, method := range authMethods {
		choices[i] = interact.Choice{
			Display: method.DisplayName,
			Value:   method,
		}
	}

	if len(choices) == 0 {
		chosenMethod = authMethod{
			Type: "none",
		}
	}

	if len(choices) == 1 {
		chosenMethod = authMethods[0]
	}

	if len(choices) > 1 {
		err = interact.NewInteraction("choose an auth method", choices...).Resolve(&chosenMethod)
		if err != nil {
			return "", "", err
		}

		fmt.Println("")
	}

	switch chosenMethod.Type {
	case "oauth":
		var tokenStr string

		stdinChannel := make(chan string)
		tokenChannel := make(chan string)
		errorChannel := make(chan error)
		portChannel := make(chan string)

		go listenForTokenCallback(tokenChannel, errorChannel, portChannel, target.Client().URL())

		port := <-portChannel

		theURL := fmt.Sprintf("%s&fly_local_port=%s\n", chosenMethod.AuthURL, port)

		fmt.Println("navigate to the following URL in your browser:")
		fmt.Println("")
		fmt.Printf("    %s", theURL)
		fmt.Println("")

		if command.OpenBrowser {
			// try to open the browser window, but don't get all hung up if it
			// fails, since we already printed about it.
			_ = open.Start(theURL)
		}

		go waitForTokenInput(stdinChannel, errorChannel)

		select {
		case tokenStrMsg := <-tokenChannel:
			tokenStr = tokenStrMsg
		case tokenStrMsg := <-stdinChannel:
			tokenStr = tokenStrMsg
		case errorMsg := <-errorChannel:
			return "", "", errorMsg
		}

		segments := strings.SplitN(tokenStr, " ", 2)

		return segments[0], segments[1], nil

	case "basic":
		var username string
		if command.Username != "" {
			username = command.Username
		} else {
			err := interact.NewInteraction("username").Resolve(interact.Required(&username))
			if err != nil {
				return "", "", err
			}
		}

		var password string
		if command.Password != "" {
			password = command.Password
		} else {
			var interactivePassword interact.Password
			err := interact.NewInteraction("password").Resolve(interact.Required(&interactivePassword))
			if err != nil {
				return "", "", err
			}
			password = string(interactivePassword)
		}

		request, err := http.NewRequest("GET", target.URL()+"/api/v1/teams/"+target.Team().Name()+"/auth/token", nil)
		if err != nil {
			return "", "", err
		}
		request.SetBasicAuth(username, password)

		tokenResponse, err := httpClient.Do(request)
		if err != nil {
			return "", "", err
		}

		type authToken struct {
			Type  string `json:"token_type"`
			Value string `json:"token_value"`
		}

		defer tokenResponse.Body.Close()

		var token authToken
		json.NewDecoder(tokenResponse.Body).Decode(&token)

		return token.Type, token.Value, nil

	case "none":
		request, err := http.NewRequest("GET", target.URL()+"/api/v1/teams/"+target.Team().Name()+"/auth/token", nil)
		if err != nil {
			return "", "", err
		}

		tokenResponse, err := httpClient.Do(request)
		if err != nil {
			return "", "", err
		}

		type authToken struct {
			Type  string `json:"token_type"`
			Value string `json:"token_value"`
		}

		defer tokenResponse.Body.Close()

		var token authToken
		json.NewDecoder(tokenResponse.Body).Decode(&token)

		return token.Type, token.Value, nil
	}

	return "", "", nil
}

package helpers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/concourse/atc"
)

type LoginInformation struct {
	NoAuth         bool
	OauthToken     string
	BasicAuthCreds basicAuthCredentials
}

type basicAuthCredentials struct {
	Username string
	Password string
}

type providerType string

const (
	githubProvider = "GitHub"
)

type oauthAuthCredentials struct {
	Provider providerType
	Username string
	Password string
}

func SetupLoginInformation(atcURL string) (LoginInformation, error) {
	dev, basicAuth, oauth, err := GetAuthMethods(atcURL)
	if err != nil {
		return LoginInformation{}, err
	}

	switch {
	case dev:
		return LoginInformation{NoAuth: true}, nil
	case basicAuth != nil:
		return LoginInformation{BasicAuthCreds: *basicAuth}, nil
	case oauth != nil:
		oauthToken, err := OauthToken(atcURL, oauth)
		if err != nil {
			return LoginInformation{}, err
		}

		return LoginInformation{OauthToken: oauthToken}, nil
	}

	return LoginInformation{}, errors.New("Unable to determine authentication")
}

func GetAuthMethods(atcURL string) (bool, *basicAuthCredentials, *oauthAuthCredentials, error) {
	endpoint := fmt.Sprintf("%s/api/v1/teams/main/auth/methods", atcURL)
	response, err := http.Get(endpoint)
	if err != nil {
		return false, nil, nil, err
	}

	var authMethods []atc.AuthMethod
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return false, nil, nil, err
	}

	err = json.Unmarshal(body, &authMethods)
	if err != nil {
		return false, nil, nil, err
	}

	var noAuth bool
	var basicCred *basicAuthCredentials
	var gitHubCred *oauthAuthCredentials

	if len(authMethods) > 0 {
		for _, auth := range authMethods {
			switch auth.Type {
			case atc.AuthTypeBasic:
				username := os.Getenv("BASIC_AUTH_USERNAME")
				password := os.Getenv("BASIC_AUTH_PASSWORD")
				if username == "" || password == "" {
					return false, nil, nil, errors.New("must set $BASIC_AUTH_USERNAME and $BASIC_AUTH_PASSWORD for basic auth")
				}

				basicCred = &basicAuthCredentials{
					Username: username,
					Password: password,
				}
			case atc.AuthTypeOAuth:
				if auth.DisplayName == githubProvider {
					username := os.Getenv("GITHUB_AUTH_USERNAME")
					password := os.Getenv("GITHUB_AUTH_PASSWORD")
					if username == "" || password == "" {
						return false, nil, nil, errors.New("must set $GITHUB_AUTH_USERNAME and $GITHUB_AUTH_PASSWORD for github auth")
					}

					gitHubCred = &oauthAuthCredentials{
						Provider: githubProvider,
						Username: username,
						Password: password,
					}
				}
			}
		}
	} else {
		noAuth = true
	}

	return noAuth, basicCred, gitHubCred, nil
}

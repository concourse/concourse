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

type basicAuthCred struct {
	Username string
	Password string
}

type gitHubAuthCred struct{}

func GetAuthMethods(atcURL string) (bool, *basicAuthCred, *gitHubAuthCred, error) {
	endpoint := fmt.Sprintf("%s/api/v1/auth/methods", atcURL)
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
	var basicCred *basicAuthCred
	var gitHubCred *gitHubAuthCred

	if len(authMethods) > 0 {
		for _, auth := range authMethods {
			switch auth.Type {
			case "basic":
				username := os.Getenv("BASIC_AUTH_USERNAME")
				password := os.Getenv("BASIC_AUTH_PASSWORD")
				if username == "" || password == "" {
					return false, nil, nil, errors.New("Need both username and password for basic auth")
				}

				basicCred = &basicAuthCred{
					Username: username,
					Password: password,
				}
			case "oauth":
				gitHubCred = &gitHubAuthCred{}
			}
		}
	} else {
		noAuth = true
	}

	return noAuth, basicCred, gitHubCred, nil
}

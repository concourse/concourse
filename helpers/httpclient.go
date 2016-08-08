package helpers

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"golang.org/x/oauth2"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse"
)

func ConcourseClient(atcURL string) concourse.Client {
	token, _ := GetATCToken(atcURL)
	httpClient := oauthClient(token)
	return concourse.NewClient(atcURL, httpClient)
}

func GetATCToken(atcURL string) (*atc.AuthToken, error) {
	response, err := http.Get(atcURL + "/api/v1/teams/main/auth/token")
	if err != nil {
		return nil, err
	}

	var token *atc.AuthToken
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, &token)
	if err != nil {
		return nil, err
	}

	return token, nil
}

func oauthClient(atcToken *atc.AuthToken) *http.Client {
	return &http.Client{
		Transport: &oauth2.Transport{
			Source: oauth2.StaticTokenSource(&oauth2.Token{
				TokenType:   atcToken.Type,
				AccessToken: atcToken.Value,
			}),
			Base: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

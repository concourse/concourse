package helpers

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"golang.org/x/oauth2"

	"github.com/concourse/go-concourse/concourse"
	"github.com/concourse/skymarshal/auth"
	"github.com/concourse/skymarshal/provider"
)

func ConcourseClient(atcURL string) concourse.Client {
	authToken, _, _ := GetATCToken(atcURL)
	httpClient := oauthClient(authToken)
	return concourse.NewClient(atcURL, httpClient, false)
}

func GetATCToken(atcURL string) (*provider.AuthToken, string, error) {
	response, err := httpClient().Get(atcURL + "/auth/basic/token?team_name=main")
	if err != nil {
		return nil, "", err
	}

	var authToken *provider.AuthToken
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, "", err
	}

	err = json.Unmarshal(body, &authToken)
	if err != nil {
		return nil, "", err
	}

	csrfToken := response.Header.Get(auth.CSRFHeaderName)

	return authToken, csrfToken, nil
}

func oauthClient(atcToken *provider.AuthToken) *http.Client {
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

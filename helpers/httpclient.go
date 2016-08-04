package helpers

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"golang.org/x/oauth2"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse"
)

func ConcourseClient(atcURL string, loginInfo LoginInformation) concourse.Client {

	var httpClient *http.Client
	switch {
	case loginInfo.NoAuth || loginInfo.BasicAuthCreds.Username != "":
		token, _ := GetATCToken(atcURL)
		httpClient = oauthClient(token)
	case loginInfo.OauthToken != "":
		httpClient = oauthClientFromString(loginInfo.OauthToken)
	}

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

func oauthClientFromString(oauthToken string) *http.Client {
	splitToken := strings.Split(oauthToken, " ")
	token := &atc.AuthToken{
		Type:  splitToken[0],
		Value: splitToken[1],
	}
	return oauthClient(token)
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

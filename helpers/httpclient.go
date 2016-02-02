package helpers

import (
	"crypto/tls"
	"net/http"
	"strings"

	"golang.org/x/oauth2"

	"github.com/concourse/go-concourse/concourse"
)

func ConcourseClient(atcURL string, loginInfo LoginInformation) concourse.Client {
	var httpClient *http.Client
	switch {
	case loginInfo.NoAuth:
	case loginInfo.BasicAuthCreds.Username != "":
		httpClient = basicAuthClient(loginInfo.BasicAuthCreds)
	case loginInfo.OauthToken != "":
		httpClient = oauthClient(loginInfo.OauthToken)
	}

	return concourse.NewClient(atcURL, httpClient)
}

func basicAuthClient(basicAuth basicAuthCredentials) *http.Client {
	return &http.Client{
		Transport: basicAuthTransport{
			username: basicAuth.Username,
			password: basicAuth.Password,
			base:     http.DefaultTransport,
		},
	}
}

func oauthClient(oauthToken string) *http.Client {
	splitToken := strings.Split(oauthToken, " ")
	token := &oauth2.Token{
		TokenType:   splitToken[0],
		AccessToken: splitToken[1],
	}

	return &http.Client{
		Transport: &oauth2.Transport{
			Source: oauth2.StaticTokenSource(token),
			Base: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
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

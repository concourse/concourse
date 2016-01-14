package helpers

import (
	"crypto/tls"
	"errors"
	"net/http"
	"strings"

	"golang.org/x/oauth2"

	"github.com/concourse/fly/rc"
	"github.com/concourse/go-concourse/concourse"
)

func ConcourseClient(atcURL string, loginInfo LoginInformation) (concourse.Client, error) {
	var httpClient *http.Client
	var err error
	switch {
	case loginInfo.NoAuth:
	case loginInfo.BasicAuthCreds.Username != "":
		httpClient, err = basicAuthClient(atcURL, loginInfo.BasicAuthCreds)
	case loginInfo.OauthToken != "":
		httpClient, err = oauthClient(loginInfo.OauthToken)
	}
	if err != nil {
		return nil, err
	}

	if !loginInfo.NoAuth && httpClient == nil {
		return nil, errors.New("Unable to determine authentication")
	}

	conn, err := concourse.NewConnection(atcURL, httpClient)
	if err != nil {
		return nil, err
	}

	return concourse.NewClient(conn), nil
}

func basicAuthClient(atcURL string, basicAuth basicAuthCredentials) (*http.Client, error) {
	newUnauthedClient, err := rc.NewConnection(atcURL, false)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: basicAuthTransport{
			username: basicAuth.Username,
			password: basicAuth.Password,
			base:     newUnauthedClient.HTTPClient().Transport,
		},
	}

	return client, nil
}

func oauthClient(oauthToken string) (*http.Client, error) {
	splitToken := strings.Split(oauthToken, " ")
	token := &oauth2.Token{
		TokenType:   splitToken[0],
		AccessToken: splitToken[1],
	}

	httpClient := &http.Client{
		Transport: &oauth2.Transport{
			Source: oauth2.StaticTokenSource(token),
			Base: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}

	return httpClient, nil
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

package helpers

import (
	"context"
	"crypto/tls"
	"net/http"

	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/go-concourse/concourse"
	"golang.org/x/oauth2"
)

func ConcourseClient(atcURL string, username, password string) concourse.Client {
	token, err := fetchToken(atcURL, username, password)
	Expect(err).NotTo(HaveOccurred())

	httpClient := &http.Client{
		Transport: &oauth2.Transport{
			Source: oauth2.StaticTokenSource(token),
			Base: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}

	return concourse.NewClient(atcURL, httpClient, false)
}

func fetchToken(atcURL string, username, password string) (*oauth2.Token, error) {

	oauth2Config := oauth2.Config{
		ClientID:     "fly",
		ClientSecret: "Zmx5",
		Endpoint:     oauth2.Endpoint{TokenURL: atcURL + "/sky/token"},
		Scopes:       []string{"openid", "federated:id"},
	}

	return oauth2Config.PasswordCredentialsToken(context.Background(), username, password)
}

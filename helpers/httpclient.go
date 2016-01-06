package helpers

import (
	"errors"
	"net/http"

	"github.com/concourse/fly/rc"
)

func GetAuthenticatedHttpClient(atcURL string) (*http.Client, error) {
	dev, basicAuth, _, err := getAuthMethods(atcURL)
	if err != nil {
		return nil, err
	}

	if dev {
		return nil, nil
	} else if basicAuth != nil {
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

	return nil, errors.New("Unable to determine authentication")
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

package cloud

import (
	"github.com/concourse/skymarshal/verifier"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"net/http"
)

type Provider struct {
	*oauth2.Config
	verifier.Verifier
}

func (p Provider) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) (string, error) {
	return p.Config.AuthCodeURL(state, opts...), nil
}

func (p Provider) Exchange(ctx context.Context, req *http.Request) (*oauth2.Token, error) {
	return p.Config.Exchange(ctx, req.FormValue("code"))
}

func (Provider) PreTokenClient() (*http.Client, error) {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			DisableKeepAlives: true,
		},
	}, nil
}

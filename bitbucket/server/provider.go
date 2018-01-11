package server

import (
	"github.com/concourse/skymarshal/verifier"
	"github.com/dghubble/oauth1"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"net/http"
	"time"
)

type Provider struct {
	*oauth1.Config
	verifier.Verifier
	secrets map[string]string
}

func (p *Provider) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) (string, error) {
	requestToken, requestSecret, err := p.Config.RequestToken()
	if err != nil {
		return "", err
	}
	authorizationURL, err := p.Config.AuthorizationURL(requestToken)
	if err != nil {
		return "", err
	}
	p.secrets[requestToken] = requestSecret
	return authorizationURL.String(), nil
}

func (p *Provider) Client(ctx context.Context, t *oauth2.Token) *http.Client {
	return p.Config.Client(ctx, &oauth1.Token{Token: t.AccessToken, TokenSecret: t.RefreshToken})
}

func (p *Provider) Exchange(ctx context.Context, req *http.Request) (*oauth2.Token, error) {
	requestToken, verifier, err := oauth1.ParseAuthorizationCallback(req)
	if err != nil {
		return nil, err
	}

	requestSecret := p.secrets[requestToken]

	accessToken, accessSecret, err := p.Config.AccessToken(requestToken, requestSecret, verifier)
	if err != nil {
		return nil, err
	}

	token := oauth1.NewToken(accessToken, accessSecret)

	return &oauth2.Token{
		AccessToken:  token.Token,
		TokenType:    "Bearer",
		RefreshToken: token.TokenSecret,
		Expiry:       time.Time{},
	}, nil
}

func (*Provider) PreTokenClient() (*http.Client, error) {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			DisableKeepAlives: true,
		},
	}, nil
}

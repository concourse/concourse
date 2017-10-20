package bitbucketserver

import (
	"github.com/concourse/atc/auth/verifier"
	"github.com/dghubble/oauth1"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"net/http"
	"time"
)

type BitbucketServerProvider struct {
	*oauth1.Config
	verifier.Verifier
	secrets map[string]string
}

func (p *BitbucketServerProvider) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	requestToken, requestSecret, err := p.Config.RequestToken()
	if err != nil {
		panic(err)
	}
	authorizationURL, err := p.Config.AuthorizationURL(requestToken)
	if err != nil {
		panic(err)
	}
	p.secrets[requestToken] = requestSecret
	return authorizationURL.String()
}

func (p *BitbucketServerProvider) Client(ctx context.Context, t *oauth2.Token) *http.Client {
	return p.Config.Client(ctx, &oauth1.Token{Token: t.AccessToken, TokenSecret: t.RefreshToken})
}

func (p *BitbucketServerProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	r := ctx.Value("request").(*http.Request)
	requestToken, verifier, err := oauth1.ParseAuthorizationCallback(r)
	if err != nil {
		panic(err)
	}

	requestSecret := p.secrets[requestToken]

	accessToken, accessSecret, err := p.Config.AccessToken(requestToken, requestSecret, verifier)
	if err != nil {
		panic(err)
	}

	token := oauth1.NewToken(accessToken, accessSecret)

	return &oauth2.Token{
		AccessToken:  token.Token,
		TokenType:    "oauth1",
		RefreshToken: token.TokenSecret,
		Expiry:       time.Time{},
	}, nil
}

func (*BitbucketServerProvider) PreTokenClient() (*http.Client, error) {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			DisableKeepAlives: true,
		},
	}, nil
}

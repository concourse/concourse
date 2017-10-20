package bitbucketserver

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/concourse/atc"
	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/auth/routes"
	"github.com/concourse/atc/auth/verifier"
	"github.com/dghubble/oauth1"
	"github.com/hashicorp/go-multierror"
	"github.com/jessevdk/go-flags"
	"github.com/tedsuo/rata"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"net/http"
	"strings"
	"time"
)

const ProviderName = "bitbucket-server"
const DisplayName = "Bitbucket Server"

var Scopes = []string{"team"}

type BitbucketServerAuthConfig struct {
	ConsumerKey string `json:"consumer_key" long:"consumer-key" description:"Application consumer key for enabling Bitbucket OAuth"`
	PrivateKey  string `json:"private_key" long:"private-key" description:"Application private key for enabling Bitbucket OAuth"`
	Endpoint    string `json:"endpoint" long:"endpoint" description:"Endpoint for Bitbucket Server"`

	Users []string `json:"users" long:"user"`
}

func (auth *BitbucketServerAuthConfig) IsConfigured() bool {
	return auth.ConsumerKey != "" ||
		auth.PrivateKey != "" ||
		auth.Endpoint != "" ||
		len(auth.Users) > 0
}

func (auth *BitbucketServerAuthConfig) Validate() error {
	var errs *multierror.Error
	if auth.Endpoint == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specifiy --bitbucket-server-auth-endpoint to OAuth with Bitbucket Server"),
		)
	}
	if auth.ConsumerKey == "" || auth.PrivateKey == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --bitbucket-server-auth-consumer-key and --bitbucket-server-auth-private-key to use OAuth with Bitbucket Server"),
		)
	}
	if len(auth.Users) == 0 {
		errs = multierror.Append(
			errs,
			errors.New("at least one of the following is required for bitbucket-server-auth: users"),
		)
	}
	return errs.ErrorOrNil()
}

func (auth *BitbucketServerAuthConfig) AuthMethod(oauthBaseURL string, teamName string) atc.AuthMethod {
	path, err := routes.OAuth1Routes.CreatePathForRoute(
		routes.OAuth1Begin,
		rata.Params{"provider": ProviderName},
	)
	if err != nil {
		panic("failed to construct oauth begin handler route: " + err.Error())
	}

	path = path + fmt.Sprintf("?team_name=%s", teamName)

	return atc.AuthMethod{
		Type:        atc.AuthTypeOAuth,
		DisplayName: DisplayName,
		AuthURL:     oauthBaseURL + path,
	}
}

func init() {
	provider.Register(ProviderName, BitbucketTeamProvider{})
}

type BitbucketProvider struct {
	*oauth1.Config
	verifier.Verifier
	secrets map[string]string
}

func (p *BitbucketProvider) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
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

func (p *BitbucketProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
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

func (p *BitbucketProvider) Client(ctx context.Context, t *oauth2.Token) *http.Client {
	return p.Config.Client(ctx, &oauth1.Token{Token: t.AccessToken, TokenSecret: t.RefreshToken})
}

func (*BitbucketProvider) PreTokenClient() (*http.Client, error) {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			DisableKeepAlives: true,
		},
	}, nil
}

type BitbucketTeamProvider struct {
}

func (BitbucketTeamProvider) ProviderConstructor(config provider.AuthConfig, redirectURL string) (provider.Provider, bool) {
	bitbucketAuth := config.(*BitbucketServerAuthConfig)

	block, _ := pem.Decode([]byte(bitbucketAuth.PrivateKey))

	var rsa *rsa.PrivateKey
	var err error
	switch block.Type {
	case "RSA PRIVATE KEY":
		rsa, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, false
		}
	}

	endpoint := oauth1.Endpoint{
		RequestTokenURL: strings.TrimRight(bitbucketAuth.Endpoint, "/") + "/plugins/servlet/oauth/request-token",
		AuthorizeURL:    strings.TrimRight(bitbucketAuth.Endpoint, "/") + "/plugins/servlet/oauth/authorize",
		AccessTokenURL:  strings.TrimRight(bitbucketAuth.Endpoint, "/") + "/plugins/servlet/oauth/access-token",
	}

	return &BitbucketProvider{
		Verifier: verifier.NewVerifierBasket(
			NewUserVerifier(bitbucketAuth.Users),
		),
		Config: &oauth1.Config{
			ConsumerKey: bitbucketAuth.ConsumerKey,
			CallbackURL: redirectURL,
			Endpoint:    endpoint,
			Signer: &oauth1.RSASigner{
				PrivateKey: rsa,
			},
		},
		secrets: make(map[string]string),
	}, true
}

func (BitbucketTeamProvider) AddAuthGroup(group *flags.Group) provider.AuthConfig {
	flags := &BitbucketServerAuthConfig{}

	bGroup, err := group.AddGroup("Bitbucket Server Authentication", "", flags)
	if err != nil {
		panic(err)
	}

	bGroup.Namespace = "bitbucket-server-auth"

	return flags
}

func (BitbucketTeamProvider) UnmarshalConfig(config *json.RawMessage) (provider.AuthConfig, error) {
	flags := &BitbucketServerAuthConfig{}
	if config != nil {
		err := json.Unmarshal(*config, &flags)
		if err != nil {
			return nil, err
		}
	}
	return flags, nil
}

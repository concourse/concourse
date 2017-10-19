package bitbucketserver

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/concourse/atc"
	"github.com/concourse/atc/auth/genericoauth"
	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/auth/routes"
	"github.com/concourse/atc/auth/verifier"
	"github.com/dghubble/oauth1"
	"github.com/hashicorp/go-multierror"
	"github.com/jessevdk/go-flags"
	"github.com/tedsuo/rata"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/bitbucket"
	"net/http"
	"time"
)

const ProviderName = "bitbucket-server"
const DisplayName = "Bitbucket Server"

var Scopes = []string{"team"}

type BitbucketAuthConfig struct {
	ConsumerKey    string `json:"consumer_key" long:"consumer-key" description:"Application client ID for enabling Bitbucket OAuth"`
	ConsumerSecret string `json:"consumer_secret" long:"consumer-secret" description:"Application client secret for enabling Bitbucket OAuth"`

	AuthURL  string `json:"auth_url,omitempty" long:"auth-url" description:"Override default endpoint AuthURL for Bitbucket Server"`
	TokenURL string `json:"token_url,omitempty" long:"token-url" description:"Override default endpoint TokenURL for Bitbucket Server"`
	APIURL   string `json:"apiurl,omitempty" long:"api-url" description:"Override default API endpoint URL for Bitbucket Server"`
}

func (auth *BitbucketAuthConfig) IsConfigured() bool {
	return auth.ConsumerKey != "" ||
		auth.ConsumerSecret != ""
}

func (auth *BitbucketAuthConfig) Validate() error {
	var errs *multierror.Error
	if auth.ConsumerKey == "" || auth.ConsumerSecret == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --bitbucket-server-auth-client-id and --bitbucket-server-auth-client-secret to use Bitbucket OAuth"),
		)
	}
	return errs.ErrorOrNil()
}

func (auth *BitbucketAuthConfig) AuthMethod(oauthBaseURL string, teamName string) atc.AuthMethod {
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
	bitbucketAuth := config.(*BitbucketAuthConfig)

	endpoint := bitbucket.Endpoint
	if bitbucketAuth.AuthURL != "" && bitbucketAuth.TokenURL != "" {
		endpoint.AuthURL = bitbucketAuth.AuthURL
		endpoint.TokenURL = bitbucketAuth.TokenURL
	}

	block, _ := pem.Decode(
		[]byte(`-----BEGIN RSA PRIVATE KEY-----

-----END RSA PRIVATE KEY-----`),
	)

	var rsa *rsa.PrivateKey
	var err error
	switch block.Type {
	case "RSA PRIVATE KEY":
		rsa, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			panic(err)
		}
	}

	return &BitbucketProvider{
		Verifier: verifier.NewVerifierBasket(
			&genericoauth.NoopVerifier{},
		),
		Config: &oauth1.Config{
			ConsumerKey: bitbucketAuth.ConsumerKey,
			CallbackURL: redirectURL,
			Endpoint: oauth1.Endpoint{
				RequestTokenURL: "http://192.168.46.253:7990/plugins/servlet/oauth/request-token",
				AuthorizeURL:    "http://192.168.46.253:7990/plugins/servlet/oauth/authorize",
				AccessTokenURL:  "http://192.168.46.253:7990/plugins/servlet/oauth/access-token",
			},
			Signer: &oauth1.RSASigner{
				PrivateKey: rsa,
			},
		},
		secrets: make(map[string]string),
	}, true
}

func (BitbucketTeamProvider) AddAuthGroup(group *flags.Group) provider.AuthConfig {
	flags := &BitbucketAuthConfig{}

	bGroup, err := group.AddGroup("Bitbucket Server Authentication", "", flags)
	if err != nil {
		panic(err)
	}

	bGroup.Namespace = "bitbucket-server-auth"

	return flags
}

func (BitbucketTeamProvider) UnmarshalConfig(config *json.RawMessage) (provider.AuthConfig, error) {
	flags := &BitbucketAuthConfig{}
	if config != nil {
		err := json.Unmarshal(*config, &flags)
		if err != nil {
			return nil, err
		}
	}
	return flags, nil
}

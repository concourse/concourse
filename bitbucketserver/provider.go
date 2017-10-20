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
	"golang.org/x/oauth2/bitbucket"
	"net/http"
	"time"
)

const ProviderName = "bitbucket-server"
const DisplayName = "Bitbucket Server"

var Scopes = []string{"team"}

type BitbucketAuthConfig struct {
	ConsumerKey string `json:"consumer_key" long:"consumer-key" description:"Application consumer key for enabling Bitbucket OAuth"`
	PrivateKey  string `json:"private_key" long:"private-key" description:"Application private key for enabling Bitbucket OAuth"`

	Users []string `json:"users" long:"user"`

	AuthURL  string `json:"auth_url,omitempty" long:"auth-url" description:"Override default endpoint AuthURL for Bitbucket Server"`
	TokenURL string `json:"token_url,omitempty" long:"token-url" description:"Override default endpoint TokenURL for Bitbucket Server"`
	APIURL   string `json:"apiurl,omitempty" long:"api-url" description:"Override default API endpoint URL for Bitbucket Server"`
}

func (auth *BitbucketAuthConfig) IsConfigured() bool {
	return auth.ConsumerKey != "" ||
		auth.PrivateKey != "" ||
		len(auth.Users) > 0
}

func (auth *BitbucketAuthConfig) Validate() error {
	var errs *multierror.Error
	if auth.ConsumerKey == "" || auth.PrivateKey == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --bitbucket-server-auth-consumer-key and --bitbucket-server-auth-private-key to use Bitbucket OAuth"),
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
MIIEpAIBAAKCAQEAwNO+7RswMf3LQYWvRHwN4Jyy8SaRphzH/+Wklkln+Wxes2sG
AIh8Tgj5yoAixfSZDInhAFa7rri5n6babpufJSPLfyfml9+I/rW/7hCPeafRQL8S
MERKMsvDFJyV0EwNMA58C1aN3O10wFuMs8wpT5sAo+5+uRBPA23kcG3xFtRUtZQW
3WDHUyXgOseZRCtSOqruIKaaV31CfjpMLk8RxNjGRlfstDrblaEX8CNJuj1LckKI
x6tNuxJDAAOSYMveZ38vqx8b6dFqdamZmW4u+Nx2WTWAncMfid0naComZk4S4fnA
+uhryT0phjjhFCqbuv3gimeFBh0qr4Qou1zdgwIDAQABAoIBAEtEG5lXbHeG9giM
Uv5rYctTvvEsOdvaDiMPky/qVUBhkZF86+nXXJXlIQNvAqO8NuVTCFVmhXnMtv/f
VBGqgvMvRqZKf9K2OTYa4WDea/Jzk9Uu/72BWmj7ahkoib21gcxJSxft4A/lTBYt
Zf1kapedDCHw3NwFxqGzCmDsORfMeIew+9VfnpY6yTjLhnadiEMGqNK90Zfh9FYR
VYWxvn9QSFFNCJ6fL1q00gteDtErJvRTqjAeEnskDZqSDO2NBZyxi/ugoue7ados
m16JFsvRTYnjSV5so/ZoI3z+xnvGUKr5xpuDzwgjF3jG4O2rdd/p9FRawCB0ulTn
l4bdSrkCgYEA/fdg2tlsRU+Ciif2v6viFHqxyUO18AbsPVw+qaYx++t+XTY4l9N2
SHXxlSN0/Bb8/+VHWOzEhEaCovyGLtvzZ9so1zWA++u+BXR3EhD0XfuWv5z5mqnz
1b1qXNH/h6etGSjUXIfYFmJyLaS/DDfQ3a8iGV0WyOKLb5T8IVEUYYUCgYEAwl8I
zDV6Mo6HArxfyyH6dDb46lNaAAgEZLvFN/ZaTxIoU7D334Eb3BbJiGG9kBCSlcBA
yI+DUS8ViXh9dry3r+dSvwD5k9hDtu36gJ3WTTuEfKFYhFUzsF1BPuGuDYgHweD9
v0OTC46pSGYcYAS/JGYG8pidPKRqlX1JEWNUbWcCgYEA20WjElF28cDcbHxkxsiY
wiXNKoCTrVHM1o22bLNZpLCGweP2qN+i2J08oA+lCaKvfiFvoI+MfMiEMkTldb/i
QGEwud8wJlI8FmmgBLEuy5ZVacsWlzr1lC2ej9WgUnerNHXUJLAFGg6VlmMPsHTg
mQaE4nFFIty2lviDWCCxACECgYB0ZZDRKV0qFWwIWWJMNObU3W6mdI+64RIweLmb
z605GLiJlbp6X8idPhAl2dI5CZOelei1siuDXFzbXApWJqEhd7d3pk/PF31FeLHA
f8Srr26ha8WkSZmQjefaji867zEmC2QpO4A9NYtuTafEYFNOqsKSWI4gmJ0zNDmj
bgZLFQKBgQDFEPhJMC2w5bu/pLHKeSwDv6bXEh8H6gyDH9YrZ6rQNUZfuCpQ28SW
9LvApjnjslrWmDI7iTCyC7uK9dvuOhds8gMFhDTR35xdD+KzTzpFKdBUpGx68HuL
zAuk6MUBKdRGr20AydS3+vQ36qx27zmf5mf0VbBAWe+rIbLT90a4Dw==
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
			NewUserVerifier(bitbucketAuth.Users),
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

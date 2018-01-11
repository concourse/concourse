package genericoauth

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"

	"encoding/json"

	"github.com/concourse/skymarshal/auth"
	"github.com/concourse/skymarshal/provider"
	"github.com/concourse/skymarshal/verifier"
	"github.com/hashicorp/go-multierror"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/rata"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

const ProviderName = "oauth"

type Provider struct {
	verifier.Verifier
	Config ConfigOverride
	CACert string
}

type ConfigOverride struct {
	oauth2.Config
	AuthURLParams map[string]string
}

type NoopVerifier struct{}

func init() {
	provider.Register(ProviderName, GenericTeamProvider{})
}

type GenericOAuthConfig struct {
	DisplayName  string `json:"display_name"      long:"display-name"    description:"Name for this auth method on the web UI."`
	ClientID     string `json:"client_id"         long:"client-id"       description:"Application client ID for enabling generic OAuth."`
	ClientSecret string `json:"client_secret"     long:"client-secret"   description:"Application client secret for enabling generic OAuth."`

	AuthURL       string                `json:"auth_url,omitempty"          long:"auth-url"        description:"Generic OAuth provider AuthURL endpoint."`
	AuthURLParams map[string]string     `json:"auth_url_params,omitempty"   long:"auth-url-param"  description:"Parameter to pass to the authentication server AuthURL. Can be specified multiple times."`
	Scope         string                `json:"scope,omitempty"             long:"scope"           description:"Optional scope required to authorize user"`
	TokenURL      string                `json:"token_url,omitempty"         long:"token-url"       description:"Generic OAuth provider TokenURL endpoint."`
	CACert        auth.FileContentsFlag `json:"ca_cert,omitempty"           long:"ca-cert"         description:"PEM-encoded CA certificate string"`
}

func (config *GenericOAuthConfig) AuthMethod(oauthBaseURL string, teamName string) provider.AuthMethod {
	path, err := auth.Routes.CreatePathForRoute(
		auth.OAuthBegin,
		rata.Params{"provider": ProviderName},
	)
	if err != nil {
		panic("failed to construct oauth begin handler route: " + err.Error())
	}

	path = path + fmt.Sprintf("?team_name=%s", teamName)

	return provider.AuthMethod{
		Type:        provider.AuthTypeOAuth,
		DisplayName: config.DisplayName,
		AuthURL:     oauthBaseURL + path,
	}
}

func (config *GenericOAuthConfig) IsConfigured() bool {
	return config.AuthURL != "" ||
		config.TokenURL != "" ||
		config.ClientID != "" ||
		config.ClientSecret != "" ||
		config.DisplayName != ""
}

func (config *GenericOAuthConfig) Validate() error {
	var errs *multierror.Error
	if config.ClientID == "" || config.ClientSecret == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --generic-oauth-client-id and --generic-oauth-client-secret to use Generic OAuth."),
		)
	}
	if config.AuthURL == "" || config.TokenURL == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --generic-oauth-auth-url and --generic-oauth-token-url to use Generic OAuth."),
		)
	}
	if config.DisplayName == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --generic-oauth-display-name to use Generic OAuth."),
		)
	}
	return errs.ErrorOrNil()
}

type GenericTeamProvider struct{}

func (GenericTeamProvider) AddAuthGroup(group *flags.Group) provider.AuthConfig {
	flags := &GenericOAuthConfig{}

	goGroup, err := group.AddGroup("Generic OAuth Authentication (allows access to ALL authenticated users)", "", flags)
	if err != nil {
		panic(err)
	}

	goGroup.Namespace = "generic-oauth"

	return flags
}

func (GenericTeamProvider) UnmarshalConfig(config *json.RawMessage) (provider.AuthConfig, error) {
	flags := &GenericOAuthConfig{}
	if config != nil {
		err := json.Unmarshal(*config, &flags)
		if err != nil {
			return nil, err
		}
	}
	return flags, nil
}

func (GenericTeamProvider) ProviderConstructor(
	config provider.AuthConfig,
	redirectURL string,
) (provider.Provider, bool) {
	genericOAuth := config.(*GenericOAuthConfig)

	endpoint := oauth2.Endpoint{}
	if genericOAuth.AuthURL != "" && genericOAuth.TokenURL != "" {
		endpoint.AuthURL = genericOAuth.AuthURL
		endpoint.TokenURL = genericOAuth.TokenURL
	}

	var oauthVerifier verifier.Verifier
	if genericOAuth.Scope != "" {
		oauthVerifier = NewScopeVerifier(genericOAuth.Scope)
	} else {
		oauthVerifier = NoopVerifier{}
	}

	return Provider{
		Verifier: oauthVerifier,
		Config: ConfigOverride{
			Config: oauth2.Config{
				ClientID:     genericOAuth.ClientID,
				ClientSecret: genericOAuth.ClientSecret,
				Endpoint:     endpoint,
				RedirectURL:  redirectURL,
			},
			AuthURLParams: genericOAuth.AuthURLParams,
		},
		CACert: string(genericOAuth.CACert),
	}, true
}

func (v NoopVerifier) Verify(logger lager.Logger, client *http.Client) (bool, error) {
	return true, nil
}

func (provider Provider) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) (string, error) {
	for key, value := range provider.Config.AuthURLParams {
		opts = append(opts, oauth2.SetAuthURLParam(key, value))

	}
	return provider.Config.AuthCodeURL(state, opts...), nil
}

func (provider Provider) Exchange(ctx context.Context, req *http.Request) (*oauth2.Token, error) {
	return provider.Config.Exchange(ctx, req.FormValue("code"))
}

func (provider Provider) Client(ctx context.Context, t *oauth2.Token) *http.Client {
	return provider.Config.Client(ctx, t)
}

func (p Provider) PreTokenClient() (*http.Client, error) {
	transport := &http.Transport{
		Proxy:             http.ProxyFromEnvironment,
		DisableKeepAlives: true,
	}

	if p.CACert != "" {
		caCertPool := x509.NewCertPool()
		ok := caCertPool.AppendCertsFromPEM([]byte(p.CACert))
		if !ok {
			return nil, errors.New("failed to use cf certificate")
		}

		transport.TLSClientConfig = &tls.Config{
			RootCAs: caCertPool,
		}
	}

	return &http.Client{
		Transport: transport,
	}, nil
}

package genericoauth_oidc

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

const ProviderName = "oauth_oidc"

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
	provider.Register(ProviderName, GenericOIDCTeamProvider{})
}

type GenericOAuthOIDCConfig struct {
	DisplayName  string `json:"display_name"      long:"display-name"    description:"Name for this auth method on the web UI."`
	ClientID     string `json:"client_id"         long:"client-id"       description:"Application client ID for enabling generic OAuth with OIDC."`
	ClientSecret string `json:"client_secret"     long:"client-secret"   description:"Application client secret for enabling generic OAuth with OIDC."`
	
	UserID       []string               `json:"user_id, omitempty"          long:"user-id"         description:"UserID required to authorize user. Can be specified multiple times."`
	Groups       []string               `json:"groups, omitempty"           long:"groups"          description:"Groups required to authorize user. Can be specified multiple times."`

	CustomGroupsName string             `json:"custom_groups_name, omitempty" long:"custom-groups-name" description:"Optional groups name to override default value returned by OIDC provider."`
	AuthURL       string                `json:"auth_url,omitempty"          long:"auth-url"        description:"Generic OAuth OIDC provider AuthURL endpoint."`
	AuthURLParams map[string]string     `json:"auth_url_params,omitempty"   long:"auth-url-param"  description:"Parameter to pass to the authentication server AuthURL. Can be specified multiple times."`
	Scope         string                `json:"scope,omitempty"             long:"scope"           description:"Optional scope required to authorize user"`
	TokenURL      string                `json:"token_url,omitempty"         long:"token-url"       description:"Generic OAuth OIDC provider TokenURL endpoint."`
	CACert        auth.FileContentsFlag `json:"ca_cert,omitempty"           long:"ca-cert"         description:"PEM-encoded CA certificate string"`
}

func (config *GenericOAuthOIDCConfig) AuthMethod(oauthBaseURL string, teamName string) provider.AuthMethod {
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

func (config *GenericOAuthOIDCConfig) IsConfigured() bool {
	return config.AuthURL != "" ||
		config.TokenURL != "" ||
		config.ClientID != "" ||
		config.ClientSecret != "" ||
		config.DisplayName != "" ||
		config.UserID != nil ||
		config.Groups != nil ||
		config.CustomGroupsName != ""
}

func (config *GenericOAuthOIDCConfig) Validate() error {
	var errs *multierror.Error
	if config.ClientID == "" || config.ClientSecret == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --generic-oauth-oidc-client-id and --generic-oauth-oidc-client-secret to use Generic OAuth OIDC."),
		)
	}
	if config.AuthURL == "" || config.TokenURL == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --generic-oauth-oidc-auth-url and --generic-oauth-oidc-token-url to use Generic OAuth OIDC."),
		)
	}
	if config.DisplayName == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --generic-oauth-oidc-display-name to use Generic OAuth OIDC."),
		)
	}
	if (config.Groups == nil || len(config.Groups) == 0) && (config.UserID == nil || len(config.UserID) == 0) {
		errs = multierror.Append(
			errs,
			errors.New("must specify either --generic-oauth-oidc-user-id or --generic-oauth-oidc-groups to use Generic OAuth OIDC."),
		)
	}
	return errs.ErrorOrNil()
}

func (config *GenericOAuthOIDCConfig) Finalize() error {
	return nil
}

type GenericOIDCTeamProvider struct{}

func (GenericOIDCTeamProvider) AddAuthGroup(group *flags.Group) provider.AuthConfig {
	flags := &GenericOAuthOIDCConfig{}

	goGroup, err := group.AddGroup("Generic OAuth OIDC Authentication (allows access to users authorized by OIDC provider)", "", flags)
	if err != nil {
		panic(err)
	}

	goGroup.Namespace = "generic-oauth-oidc"

	return flags
}

func (GenericOIDCTeamProvider) UnmarshalConfig(config *json.RawMessage) (provider.AuthConfig, error) {
	flags := &GenericOAuthOIDCConfig{}
	if config != nil {
		err := json.Unmarshal(*config, &flags)
		if err != nil {
			return nil, err
		}
	}
	return flags, nil
}

func (GenericOIDCTeamProvider) MarshalConfig(config provider.AuthConfig) (*json.RawMessage, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	return (*json.RawMessage)(&data), nil
}

func (GenericOIDCTeamProvider) ProviderConstructor(
	config provider.AuthConfig,
	args ...string,
) (provider.Provider, bool) {
	genericOAuthOIDC := config.(*GenericOAuthOIDCConfig)

	endpoint := oauth2.Endpoint{}
	if genericOAuthOIDC.AuthURL != "" && genericOAuthOIDC.TokenURL != "" {
		endpoint.AuthURL = genericOAuthOIDC.AuthURL
		endpoint.TokenURL = genericOAuthOIDC.TokenURL
	}

	var oauthVerifier verifier.Verifier
	oauthVerifier = NewGroupsVerifier(genericOAuthOIDC.UserID,
		genericOAuthOIDC.Groups,
		genericOAuthOIDC.CustomGroupsName)

	return Provider{
		Verifier: oauthVerifier,
		Config: ConfigOverride{
			Config: oauth2.Config{
				ClientID:     genericOAuthOIDC.ClientID,
				ClientSecret: genericOAuthOIDC.ClientSecret,
				Endpoint:     endpoint,
				RedirectURL:  args[0],
				Scopes:       []string {genericOAuthOIDC.Scope},
			},
			AuthURLParams: genericOAuthOIDC.AuthURLParams,
		},
		CACert: string(genericOAuthOIDC.CACert),
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


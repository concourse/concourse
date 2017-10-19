package bitbucketcloud

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/concourse/atc"
	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/auth/routes"
	"github.com/concourse/atc/auth/verifier"
	"github.com/hashicorp/go-multierror"
	"github.com/jessevdk/go-flags"
	"github.com/tedsuo/rata"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/bitbucket"
	"net/http"
	"github.com/concourse/atc/auth/genericoauth"
)

const ProviderName = "bitbucket-cloud"
const DisplayName = "Bitbucket Cloud"

var Scopes = []string{"team"}

type BitbucketAuthConfig struct {
	ClientID     string `json:"client_id" long:"client-id" description:"Application client ID for enabling Bitbucket OAuth"`
	ClientSecret string `json:"client_secret" long:"client-secret" description:"Application client secret for enabling Bitbucket OAuth"`

	AuthURL  string `json:"auth_url,omitempty" long:"auth-url" description:"Override default endpoint AuthURL for Bitbucket Server"`
	TokenURL string `json:"token_url,omitempty" long:"token-url" description:"Override default endpoint TokenURL for Bitbucket Server"`
	APIURL   string `json:"apiurl,omitempty" long:"api-url" description:"Override default API endpoint URL for Bitbucket Server"`
}

func (auth *BitbucketAuthConfig) IsConfigured() bool {
	return auth.ClientID != "" ||
		auth.ClientSecret != ""
}

func (auth *BitbucketAuthConfig) Validate() error {
	var errs *multierror.Error
	if auth.ClientID == "" || auth.ClientSecret == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --bitbucket-auth-client-id and --bitbucket-auth-client-secret to use Bitbucket OAuth"),
		)
	}
	return errs.ErrorOrNil()
}

func (auth *BitbucketAuthConfig) AuthMethod(oauthBaseURL string, teamName string) atc.AuthMethod {
	path, err := routes.OAuthRoutes.CreatePathForRoute(
		routes.OAuthBegin,
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
	*oauth2.Config
	verifier.Verifier
}

func (BitbucketProvider) PreTokenClient() (*http.Client, error) {
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

	// ...
	endpoint := bitbucket.Endpoint
	if bitbucketAuth.AuthURL != "" && bitbucketAuth.TokenURL != "" {
		endpoint.AuthURL = bitbucketAuth.AuthURL
		endpoint.TokenURL = bitbucketAuth.TokenURL
	}

	return BitbucketProvider{
		Verifier: verifier.NewVerifierBasket(
			&genericoauth.NoopVerifier{},
		),
		Config: &oauth2.Config{
			ClientID:     bitbucketAuth.ClientID,
			ClientSecret: bitbucketAuth.ClientSecret,
			Endpoint:     endpoint,
			Scopes:       Scopes,
			RedirectURL:  redirectURL,
		},
	}, true
}

func (BitbucketTeamProvider) AddAuthGroup(group *flags.Group) provider.AuthConfig {
	flags := &BitbucketAuthConfig{}

	bGroup, err := group.AddGroup("Bitbucket Cloud Authentication", "", flags)
	if err != nil {
		panic(err)
	}

	bGroup.Namespace = "bitbucket-cloud-auth"

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

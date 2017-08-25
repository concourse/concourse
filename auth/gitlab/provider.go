package gitlab

import (
	"errors"
	"net/http"

	"golang.org/x/oauth2"

	"fmt"

	"encoding/json"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/auth/routes"
	"github.com/concourse/atc/auth/verifier"
	"github.com/hashicorp/go-multierror"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/rata"
)

const ProviderName = "gitlab"
const DisplayName = "GitLab"

var Scopes = []string{"read_user", "api"}

type GitLabAuthConfig struct {
	ClientID     string `json:"client_id"     long:"client-id"     description:"Application client ID for enabling GitLab OAuth."`
	ClientSecret string `json:"client_secret" long:"client-secret" description:"Application client secret for enabling GitLab OAuth."`

	Groups   []string `json:"groups,omitempty" long:"group"  description:"GitLab group whose members will have access." value-name:"GROUP"`
	AuthURL  string   `json:"auth_url,omitempty"      long:"auth-url"      description:"Override default endpoint AuthURL for GitLab."`
	TokenURL string   `json:"token_url,omitempty"     long:"token-url"     description:"Override default endpoint TokenURL for GitLab."`
	APIURL   string   `json:"api_url,omitempty"       long:"api-url"       description:"Override default API endpoint URL for GitLab."`
}

func (*GitLabAuthConfig) AuthMethod(oauthBaseURL string, teamName string) atc.AuthMethod {
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

func (auth *GitLabAuthConfig) IsConfigured() bool {
	return auth.ClientID != "" ||
		auth.ClientSecret != "" ||
		len(auth.Groups) > 0
}

func (auth *GitLabAuthConfig) Validate() error {
	var errs *multierror.Error
	if auth.ClientID == "" || auth.ClientSecret == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --gitlab-auth-client-id and --gitlab-auth-client-secret to use GitLab OAuth."),
		)
	}
	if len(auth.Groups) == 0 {
		errs = multierror.Append(
			errs,
			errors.New("the following is required for gitlab-auth: groups"),
		)
	}
	return errs.ErrorOrNil()
}

type GitLabGroupConfig struct {
	GroupName string `json:"group_name,omitempty"`
}

type GitLabProvider struct {
	*oauth2.Config
	verifier.Verifier
}

func init() {
	provider.Register(ProviderName, GitLabTeamProvider{})
}

type GitLabTeamProvider struct {
}

func (GitLabTeamProvider) AddAuthGroup(group *flags.Group) provider.AuthConfig {
	flags := &GitLabAuthConfig{}

	ghGroup, err := group.AddGroup("GitLab Authentication", "", flags)
	if err != nil {
		panic(err)
	}

	ghGroup.Namespace = "gitlab-auth"

	return flags
}

func (GitLabTeamProvider) UnmarshalConfig(config *json.RawMessage) (provider.AuthConfig, error) {
	flags := &GitLabAuthConfig{}
	if config != nil {
		err := json.Unmarshal(*config, &flags)
		if err != nil {
			return nil, err
		}
	}
	return flags, nil
}

func (GitLabTeamProvider) ProviderConstructor(
	config provider.AuthConfig,
	redirectURL string,
) (provider.Provider, bool) {
	gitlabAuth := config.(*GitLabAuthConfig)

	client := NewClient(gitlabAuth.APIURL)

	endpoint := oauth2.Endpoint{}
	if gitlabAuth.AuthURL != "" && gitlabAuth.TokenURL != "" {
		endpoint.AuthURL = gitlabAuth.AuthURL
		endpoint.TokenURL = gitlabAuth.TokenURL
	}

	return GitLabProvider{
		Verifier: verifier.NewVerifierBasket(
			NewGroupVerifier(gitlabAuth.Groups, client),
		),
		Config: &oauth2.Config{
			ClientID:     gitlabAuth.ClientID,
			ClientSecret: gitlabAuth.ClientSecret,
			Endpoint:     endpoint,
			Scopes:       Scopes,
			RedirectURL:  redirectURL,
		},
	}, true
}

func (GitLabProvider) PreTokenClient() (*http.Client, error) {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			DisableKeepAlives: true,
		},
	}, nil
}

package gitlab

import (
	"errors"
	"net/http"

	"golang.org/x/oauth2"

	"fmt"

	"encoding/json"

	"github.com/concourse/skymarshal/auth"
	"github.com/concourse/skymarshal/provider"
	"github.com/concourse/skymarshal/verifier"
	"github.com/hashicorp/go-multierror"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/rata"
	"golang.org/x/net/context"
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

func (config *GitLabAuthConfig) AuthMethod(oauthBaseURL string, teamName string) provider.AuthMethod {
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
		DisplayName: DisplayName,
		AuthURL:     oauthBaseURL + path,
	}
}

func (config *GitLabAuthConfig) IsConfigured() bool {
	return config.ClientID != "" ||
		config.ClientSecret != "" ||
		len(config.Groups) > 0
}

func (config *GitLabAuthConfig) Validate() error {
	var errs *multierror.Error
	if config.ClientID == "" || config.ClientSecret == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --gitlab-auth-client-id and --gitlab-auth-client-secret to use GitLab OAuth."),
		)
	}
	if len(config.Groups) == 0 {
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

func (p GitLabProvider) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) (string, error) {
	return p.Config.AuthCodeURL(state, opts...), nil
}

func (p GitLabProvider) Exchange(ctx context.Context, req *http.Request) (*oauth2.Token, error) {
	return p.Config.Exchange(ctx, req.FormValue("code"))
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

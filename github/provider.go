package github

import (
	"errors"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"

	"fmt"
	"strings"

	"encoding/json"

	"github.com/concourse/skymarshal/auth"
	"github.com/concourse/skymarshal/provider"
	"github.com/concourse/skymarshal/verifier"
	"github.com/hashicorp/go-multierror"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/rata"
	"golang.org/x/net/context"
)

const ProviderName = "github"
const DisplayName = "GitHub"

var Scopes = []string{"read:org"}

type GitHubAuthConfig struct {
	ClientID     string `json:"client_id"     long:"client-id"     description:"Application client ID for enabling GitHub OAuth."`
	ClientSecret string `json:"client_secret" long:"client-secret" description:"Application client secret for enabling GitHub OAuth."`

	Organizations []string           `json:"organizations,omitempty" long:"organization"  description:"GitHub organization whose members will have access." value-name:"ORG"`
	Teams         []GitHubTeamConfig `json:"teams,omitempty"         long:"team"          description:"GitHub team whose members will have access." value-name:"ORG/TEAM"`
	Users         []string           `json:"users,omitempty"         long:"user"          description:"GitHub user to permit access." value-name:"LOGIN"`
	AuthURL       string             `json:"auth_url,omitempty"      long:"auth-url"      description:"Override default endpoint AuthURL for Github Enterprise."`
	TokenURL      string             `json:"token_url,omitempty"     long:"token-url"     description:"Override default endpoint TokenURL for Github Enterprise."`
	APIURL        string             `json:"api_url,omitempty"       long:"api-url"       description:"Override default API endpoint URL for Github Enterprise."`
}

func (config *GitHubAuthConfig) AuthMethod(oauthBaseURL string, teamName string) provider.AuthMethod {
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

func (config *GitHubAuthConfig) IsConfigured() bool {
	return config.ClientID != "" ||
		config.ClientSecret != "" ||
		len(config.Organizations) > 0 ||
		len(config.Teams) > 0 ||
		len(config.Users) > 0
}

func (config *GitHubAuthConfig) Validate() error {
	var errs *multierror.Error
	if config.ClientID == "" || config.ClientSecret == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --github-auth-client-id and --github-auth-client-secret to use GitHub OAuth."),
		)
	}
	if len(config.Organizations) == 0 && len(config.Teams) == 0 && len(config.Users) == 0 {
		errs = multierror.Append(
			errs,
			errors.New("at least one of the following is required for github-auth: organizations, teams, users."),
		)
	}
	return errs.ErrorOrNil()
}

type GitHubTeamConfig struct {
	OrganizationName string `json:"organization_name,omitempty"`
	TeamName         string `json:"team_name,omitempty"`
}

func (flag *GitHubTeamConfig) UnmarshalFlag(value string) error {
	s := strings.SplitN(value, "/", 2)
	if len(s) != 2 {
		return fmt.Errorf("malformed GitHub team specification: '%s'", value)
	}

	flag.OrganizationName = s[0]
	flag.TeamName = s[1]

	return nil
}

type GitHubProvider struct {
	*oauth2.Config
	verifier.Verifier
}

func (p GitHubProvider) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) (string, error) {
	return p.Config.AuthCodeURL(state, opts...), nil
}

func (p GitHubProvider) Exchange(ctx context.Context, req *http.Request) (*oauth2.Token, error) {
	return p.Config.Exchange(ctx, req.FormValue("code"))
}

func init() {
	provider.Register(ProviderName, GitHubTeamProvider{})
}

type GitHubTeamProvider struct {
}

func (GitHubTeamProvider) AddAuthGroup(group *flags.Group) provider.AuthConfig {
	flags := &GitHubAuthConfig{}

	ghGroup, err := group.AddGroup("Github Authentication", "", flags)
	if err != nil {
		panic(err)
	}

	ghGroup.Namespace = "github-auth"

	return flags
}

func (GitHubTeamProvider) UnmarshalConfig(config *json.RawMessage) (provider.AuthConfig, error) {
	flags := &GitHubAuthConfig{}
	if config != nil {
		err := json.Unmarshal(*config, &flags)
		if err != nil {
			return nil, err
		}
	}
	return flags, nil
}

func (GitHubTeamProvider) ProviderConstructor(
	config provider.AuthConfig,
	redirectURL string,
) (provider.Provider, bool) {
	githubAuth := config.(*GitHubAuthConfig)

	client := NewClient(githubAuth.APIURL)

	endpoint := github.Endpoint
	if githubAuth.AuthURL != "" && githubAuth.TokenURL != "" {
		endpoint.AuthURL = githubAuth.AuthURL
		endpoint.TokenURL = githubAuth.TokenURL
	}

	return GitHubProvider{
		Verifier: verifier.NewVerifierBasket(
			NewTeamVerifier(teamConfigsToTeam(githubAuth.Teams), client),
			NewOrganizationVerifier(githubAuth.Organizations, client),
			NewUserVerifier(githubAuth.Users, client),
		),
		Config: &oauth2.Config{
			ClientID:     githubAuth.ClientID,
			ClientSecret: githubAuth.ClientSecret,
			Endpoint:     endpoint,
			Scopes:       Scopes,
			RedirectURL:  redirectURL,
		},
	}, true
}

func (GitHubProvider) PreTokenClient() (*http.Client, error) {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			DisableKeepAlives: true,
		},
	}, nil
}

func teamConfigsToTeam(dbteams []GitHubTeamConfig) []Team {
	teams := []Team{}
	for _, team := range dbteams {
		teams = append(teams, Team{
			Name:         team.TeamName,
			Organization: team.OrganizationName,
		})
	}
	return teams
}

package cloud

import (
	"errors"
	"fmt"

	"github.com/concourse/skymarshal/auth"
	"github.com/concourse/skymarshal/bitbucket"
	"github.com/concourse/skymarshal/provider"
	"github.com/hashicorp/go-multierror"
	"github.com/tedsuo/rata"
)

type AuthConfig struct {
	ClientID     string `json:"client_id" long:"client-id" description:"Application client ID for enabling Bitbucket OAuth"`
	ClientSecret string `json:"client_secret" long:"client-secret" description:"Application client secret for enabling Bitbucket OAuth"`

	Users        []string                     `json:"users,omitempty" long:"user" description:"Bitbucket users that are allowed to log in" value-name:"USER"`
	Teams        []TeamConfig                 `json:"teams,omitempty" long:"team" description:"Bitbucket teams which members are allowed to log in" value-name:"TEAM[:ROLE]"`
	Repositories []bitbucket.RepositoryConfig `json:"repositories,omitempty" long:"repository" description:"Bitbucket repositories whose members are allowed to log in" value-name:"OWNER/REPO"`

	AuthURL  string `json:"auth_url,omitempty" long:"auth-url" description:"Override default endpoint AuthURL for Bitbucket Cloud"`
	TokenURL string `json:"token_url,omitempty" long:"token-url" description:"Override default endpoint TokenURL for Bitbucket Cloud"`
	APIURL   string `json:"apiurl,omitempty" long:"api-url" description:"Override default API endpoint URL for Bitbucket Cloud"`
}

func (config *AuthConfig) AuthMethod(oauthBaseURL string, teamName string) provider.AuthMethod {
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

func (config *AuthConfig) IsConfigured() bool {
	return config.ClientID != "" ||
		config.ClientSecret != "" ||
		len(config.Users) > 0 ||
		len(config.Teams) > 0 ||
		len(config.Repositories) > 0
}

func (config *AuthConfig) Validate() error {
	var errs *multierror.Error
	if config.ClientID == "" || config.ClientSecret == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --bitbucket-cloud-auth-client-id and --bitbucket-cloud-auth-client-secret to use OAuth with Bitbucket Cloud"),
		)
	}
	if len(config.Users) == 0 && len(config.Teams) == 0 && len(config.Repositories) == 0 {
		errs = multierror.Append(
			errs,
			errors.New("at least one of the following is required for bitbucket-cloud-auth: user, team, repository"),
		)
	}
	return errs.ErrorOrNil()
}

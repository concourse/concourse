package server

import (
	"errors"
	"fmt"
	"github.com/concourse/atc"
	"github.com/concourse/atc/auth/bitbucket"
	"github.com/concourse/atc/auth/routes"
	"github.com/hashicorp/go-multierror"
	"github.com/tedsuo/rata"
)

type AuthConfig struct {
	ConsumerKey string           `json:"consumer_key" long:"consumer-key" description:"Application consumer key for enabling Bitbucket OAuth"`
	PrivateKey  privateKeyConfig `json:"private_key" long:"private-key" description:"Path to application private key for enabling Bitbucket OAuth"`
	Endpoint    string           `json:"endpoint" long:"endpoint" description:"Endpoint for Bitbucket Server"`

	Users        []string                     `json:"users,omitempty" long:"user" description:"Bitbucket users that are allowed to log in" value-name:"USER"`
	Projects     []string                     `json:"projects,omitempty" long:"project" description:"Bitbucket projects whose members are allowed to log in" value-name:"PROJ"`
	Repositories []bitbucket.RepositoryConfig `json:"repositories,omitempty" long:"repository" description:"Bitbucket repositories whose members are allowed to log in" value-name:"OWNER/REPO"`
}

func (auth *AuthConfig) AuthMethod(oauthBaseURL string, teamName string) atc.AuthMethod {
	path, err := routes.OAuthV1Routes.CreatePathForRoute(
		routes.OAuthV1Begin,
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

func (auth *AuthConfig) IsConfigured() bool {
	return auth.ConsumerKey != "" ||
		auth.PrivateKey.PrivateKey != nil ||
		auth.Endpoint != "" ||
		len(auth.Users) > 0 ||
		len(auth.Projects) > 0 ||
		len(auth.Repositories) > 0
}

func (auth *AuthConfig) Validate() error {
	var errs *multierror.Error
	if auth.Endpoint == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specifiy --bitbucket-server-auth-endpoint to use OAuth with Bitbucket Server"),
		)
	}
	if auth.ConsumerKey == "" || auth.PrivateKey.PrivateKey == nil {
		errs = multierror.Append(
			errs,
			errors.New("must specify --bitbucket-server-auth-consumer-key and --bitbucket-server-auth-private-key to use OAuth with Bitbucket Server"),
		)
	}
	if len(auth.Users) == 0 && len(auth.Projects) == 0 && len(auth.Repositories) == 0 {
		errs = multierror.Append(
			errs,
			errors.New("at least one of the following is required for bitbucket-server-auth: user, project, repository"),
		)
	}
	return errs.ErrorOrNil()
}

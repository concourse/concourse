package bitbucketcloud

import (
	"errors"
	"fmt"
	"github.com/concourse/atc"
	"github.com/concourse/atc/auth/routes"
	"github.com/hashicorp/go-multierror"
	"github.com/tedsuo/rata"
)

type BitbucketCloudAuthConfig struct {
	ClientID     string `json:"client_id" long:"client-id" description:"Application client ID for enabling Bitbucket OAuth"`
	ClientSecret string `json:"client_secret" long:"client-secret" description:"Application client secret for enabling Bitbucket OAuth"`

	Users []string `json:"users,omitempty" long:"user" description:"Bitbucket users that are allowed to log in."`

	AuthURL  string `json:"auth_url,omitempty" long:"auth-url" description:"Override default endpoint AuthURL for Bitbucket Cloud"`
	TokenURL string `json:"token_url,omitempty" long:"token-url" description:"Override default endpoint TokenURL for Bitbucket Cloud"`
	APIURL   string `json:"apiurl,omitempty" long:"api-url" description:"Override default API endpoint URL for Bitbucket Cloud"`
}

func (auth *BitbucketCloudAuthConfig) AuthMethod(oauthBaseURL string, teamName string) atc.AuthMethod {
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

func (auth *BitbucketCloudAuthConfig) IsConfigured() bool {
	return auth.ClientID != "" ||
		auth.ClientSecret != "" ||
		len(auth.Users) > 0
}

func (auth *BitbucketCloudAuthConfig) Validate() error {
	var errs *multierror.Error
	if auth.ClientID == "" || auth.ClientSecret == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --bitbucket-cloud-auth-client-id and --bitbucket-cloud-auth-client-secret to use OAuth with Bitbucket Cloud"),
		)
	}
	if len(auth.Users) == 0 {
		errs = multierror.Append(
			errs,
			errors.New("at least one of the following is required for bitbucket-cloud-auth: users"),
		)
	}
	return errs.ErrorOrNil()
}

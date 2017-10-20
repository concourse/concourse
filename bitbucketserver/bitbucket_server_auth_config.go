package bitbucketserver

import (
	"errors"
	"fmt"
	"github.com/concourse/atc"
	"github.com/concourse/atc/auth/routes"
	"github.com/hashicorp/go-multierror"
	"github.com/tedsuo/rata"
)

type BitbucketServerAuthConfig struct {
	ConsumerKey string `json:"consumer_key" long:"consumer-key" description:"Application consumer key for enabling Bitbucket OAuth"`
	PrivateKey  string `json:"private_key" long:"private-key" description:"Application private key for enabling Bitbucket OAuth, in base64 encoded DER format"`
	Endpoint    string `json:"endpoint" long:"endpoint" description:"Endpoint for Bitbucket Server"`

	Users []string `json:"users" long:"user"`
}

func (auth *BitbucketServerAuthConfig) AuthMethod(oauthBaseURL string, teamName string) atc.AuthMethod {
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

func (auth *BitbucketServerAuthConfig) IsConfigured() bool {
	return auth.ConsumerKey != "" ||
		auth.PrivateKey != "" ||
		auth.Endpoint != "" ||
		len(auth.Users) > 0
}

func (auth *BitbucketServerAuthConfig) Validate() error {
	var errs *multierror.Error
	if auth.Endpoint == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specifiy --bitbucket-server-auth-endpoint to use OAuth with Bitbucket Server"),
		)
	}
	if auth.ConsumerKey == "" || auth.PrivateKey == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --bitbucket-server-auth-consumer-key and --bitbucket-server-auth-private-key to use OAuth with Bitbucket Server"),
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

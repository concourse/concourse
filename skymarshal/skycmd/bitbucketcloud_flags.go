package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/dex/connector/bitbucketcloud"
	"github.com/hashicorp/go-multierror"
)

func init() {
	RegisterConnector(&Connector{
		id:         "bitbucket-cloud",
		config:     &BitbucketCloudFlags{},
		teamConfig: &BitbucketCloudTeamFlags{},
	})
}

type BitbucketCloudFlags struct {
	ClientID     string `long:"client-id" description:"(Required) Client id"`
	ClientSecret string `long:"client-secret" description:"(Required) Client secret"`
}

func (flag *BitbucketCloudFlags) Name() string {
	return "Bitbucket Cloud"
}

func (flag *BitbucketCloudFlags) Validate() error {
	var errs *multierror.Error

	if flag.ClientID == "" {
		errs = multierror.Append(errs, errors.New("Missing client-id"))
	}

	if flag.ClientSecret == "" {
		errs = multierror.Append(errs, errors.New("Missing client-secret"))
	}

	return errs.ErrorOrNil()
}

func (flag *BitbucketCloudFlags) Serialize(redirectURI string) ([]byte, error) {
	if err := flag.Validate(); err != nil {
		return nil, err
	}

	return json.Marshal(bitbucketcloud.Config{
		ClientID:     flag.ClientID,
		ClientSecret: flag.ClientSecret,
		RedirectURI:  redirectURI,
	})
}

type BitbucketCloudTeamFlags struct {
	Users []string `long:"user" description:"A whitelisted Bitbucket Cloud user" value-name:"USERNAME"`
	Teams []string `long:"team" description:"A whitelisted Bitbucket Cloud team" value-name:"TEAM_NAME"`
}

func (flag *BitbucketCloudTeamFlags) GetUsers() []string {
	return flag.Users
}

func (flag *BitbucketCloudTeamFlags) GetGroups() []string {
	return flag.Teams
}

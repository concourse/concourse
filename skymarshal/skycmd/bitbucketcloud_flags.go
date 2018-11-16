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

func (self *BitbucketCloudFlags) Name() string {
	return "Bitbucket Cloud"
}

func (self *BitbucketCloudFlags) Validate() error {
	var errs *multierror.Error

	if self.ClientID == "" {
		errs = multierror.Append(errs, errors.New("Missing client-id"))
	}

	if self.ClientSecret == "" {
		errs = multierror.Append(errs, errors.New("Missing client-secret"))
	}

	return errs.ErrorOrNil()
}

func (self *BitbucketCloudFlags) Serialize(redirectURI string) ([]byte, error) {
	if err := self.Validate(); err != nil {
		return nil, err
	}

	return json.Marshal(bitbucketcloud.Config{
		ClientID:     self.ClientID,
		ClientSecret: self.ClientSecret,
		RedirectURI:  redirectURI,
	})
}

type BitbucketCloudTeamFlags struct {
	Users []string `long:"user" description:"List of whitelisted Bitbucket Cloud users" value-name:"USERNAME"`
	Teams []string `long:"team" description:"List of whitelisted Bitbucket Cloud teams" value-name:"TEAM_NAME"`
}

func (self *BitbucketCloudTeamFlags) IsValid() bool {
	return len(self.Users) > 0 || len(self.Teams) > 0
}

func (self *BitbucketCloudTeamFlags) GetUsers() []string {
	return self.Users
}

func (self *BitbucketCloudTeamFlags) GetGroups() []string {
	return self.Teams
}

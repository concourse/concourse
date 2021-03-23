package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/dex/connector/bitbucketcloud"
	"github.com/hashicorp/go-multierror"
)

type BitbucketCloudFlags struct {
	ClientID     string `yaml:"client_id,omitempty"`
	ClientSecret string `yaml:"client_secret,omitempty"`
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
		ClientID:          flag.ClientID,
		ClientSecret:      flag.ClientSecret,
		RedirectURI:       redirectURI,
		IncludeTeamGroups: true,
	})
}

type BitbucketCloudTeamFlags struct {
	Users []string `yaml:"users" env:"CONCOURSE_MAIN_TEAM_BITBUCKET_CLOUD_USERS,CONCOURSE_MAIN_TEAM_BITBUCKET_CLOUD_USER" long:"user" description:"A whitelisted Bitbucket Cloud user" value-name:"USERNAME"`
	Teams []string `yaml:"teams" env:"CONCOURSE_MAIN_TEAM_BITBUCKET_CLOUD_TEAMS,CONCOURSE_MAIN_TEAM_BITBUCKET_CLOUD_TEAM" long:"team" description:"A whitelisted Bitbucket Cloud team" value-name:"TEAM_NAME"`
}

func (flag *BitbucketCloudTeamFlags) ID() string {
	return BitbucketCloudConnectorID
}

func (flag *BitbucketCloudTeamFlags) GetUsers() []string {
	return flag.Users
}

func (flag *BitbucketCloudTeamFlags) GetGroups() []string {
	return flag.Teams
}

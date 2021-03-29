package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/concourse/flag"
	"github.com/concourse/dex/connector/github"
	multierror "github.com/hashicorp/go-multierror"
)

type GithubFlags struct {
	ClientID     string    `yaml:"client_id,omitempty"`
	ClientSecret string    `yaml:"client_secret,omitempty"`
	Host         string    `yaml:"host,omitempty"`
	CACert       flag.File `yaml:"ca_cert,omitempty"`
}

func (flag *GithubFlags) ID() string {
	return GithubConnectorID
}

func (flag *GithubFlags) Name() string {
	return "GitHub"
}

func (flag *GithubFlags) Validate() error {
	var errs *multierror.Error

	if flag.ClientID == "" {
		errs = multierror.Append(errs, errors.New("Missing client-id"))
	}

	if flag.ClientSecret == "" {
		errs = multierror.Append(errs, errors.New("Missing client-secret"))
	}

	return errs.ErrorOrNil()
}

func (flag *GithubFlags) Serialize(redirectURI string) ([]byte, error) {
	if err := flag.Validate(); err != nil {
		return nil, err
	}

	return json.Marshal(github.Config{
		ClientID:      flag.ClientID,
		ClientSecret:  flag.ClientSecret,
		RedirectURI:   redirectURI,
		HostName:      flag.Host,
		RootCA:        flag.CACert.Path(),
		TeamNameField: "both",
		LoadAllGroups: true,
	})
}

type GithubTeamFlags struct {
	Users []string `yaml:"users,omitempty" env:"CONCOURSE_MAIN_TEAM_GITHUB_USERS,CONCOURSE_MAIN_TEAM_GITHUB_USER" long:"user" description:"A whitelisted GitHub user" value-name:"USERNAME"`
	Orgs  []string `yaml:"orgs,omitempty" env:"CONCOURSE_MAIN_TEAM_GITHUB_ORGS,CONCOURSE_MAIN_TEAM_GITHUB_ORG" long:"org" description:"A whitelisted GitHub org" value-name:"ORG_NAME"`
	Teams []string `yaml:"teams,omitempty" env:"CONCOURSE_MAIN_TEAM_GITHUB_TEAMS,CONCOURSE_MAIN_TEAM_GITHUB_TEAM" long:"team" description:"A whitelisted GitHub team" value-name:"ORG_NAME:TEAM_NAME"`
}

func (flag *GithubTeamFlags) ID() string {
	return GithubConnectorID
}

func (flag *GithubTeamFlags) GetUsers() []string {
	return flag.Users
}

func (flag *GithubTeamFlags) GetGroups() []string {
	return append(flag.Orgs, flag.Teams...)
}

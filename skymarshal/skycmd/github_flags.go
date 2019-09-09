package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/dex/connector/github"
	"github.com/concourse/flag"
	multierror "github.com/hashicorp/go-multierror"
)

func init() {
	RegisterConnector(&Connector{
		id:         "github",
		config:     &GithubFlags{},
		teamConfig: &GithubTeamFlags{},
	})
}

type GithubFlags struct {
	ClientID     string    `long:"client-id" description:"(Required) Client id"`
	ClientSecret string    `long:"client-secret" description:"(Required) Client secret"`
	Host         string    `long:"host" description:"Hostname of GitHub Enterprise deployment (No scheme, No trailing slash)"`
	CACert       flag.File `long:"ca-cert" description:"CA certificate of GitHub Enterprise deployment"`
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
	Users []string `long:"user" description:"A whitelisted GitHub user" value-name:"USERNAME"`
	Orgs  []string `long:"org" description:"A whitelisted GitHub org" value-name:"ORG_NAME"`
	Teams []string `long:"team" description:"A whitelisted GitHub team" value-name:"ORG_NAME:TEAM_NAME"`
}

func (flag *GithubTeamFlags) GetUsers() []string {
	return flag.Users
}

func (flag *GithubTeamFlags) GetGroups() []string {
	return append(flag.Orgs, flag.Teams...)
}

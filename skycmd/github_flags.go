package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/flag"
	"github.com/coreos/dex/connector/github"
	"github.com/hashicorp/go-multierror"
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

func (self *GithubFlags) Name() string {
	return "GitHub"
}

func (self *GithubFlags) Validate() error {
	var errs *multierror.Error

	if self.ClientID == "" {
		errs = multierror.Append(errs, errors.New("Missing client-id"))
	}

	if self.ClientSecret == "" {
		errs = multierror.Append(errs, errors.New("Missing client-secret"))
	}

	return errs.ErrorOrNil()
}

func (self *GithubFlags) Serialize(redirectURI string) ([]byte, error) {
	if err := self.Validate(); err != nil {
		return nil, err
	}

	return json.Marshal(github.Config{
		ClientID:     self.ClientID,
		ClientSecret: self.ClientSecret,
		RedirectURI:  redirectURI,
		HostName:     self.Host,
		RootCA:       self.CACert.Path(),
	})
}

type GithubTeamFlags struct {
	Users []string `long:"user" description:"List of whitelisted GitHub users" value-name:"USERNAME"`
	Orgs  []string `long:"org" description:"List of whitelisted GitHub orgs" value-name:"ORG_NAME"`
	Teams []string `long:"team" description:"List of whitelisted GitHub teams" value-name:"ORG_NAME:TEAM_NAME"`
}

func (self *GithubTeamFlags) IsValid() bool {
	return len(self.Users) > 0 || len(self.Orgs) > 0 || len(self.Teams) > 0
}

func (self *GithubTeamFlags) GetUsers() []string {
	return self.Users
}

func (self *GithubTeamFlags) GetGroups() []string {
	return append(self.Orgs, self.Teams...)
}

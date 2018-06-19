package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/coreos/dex/connector/gitlab"
	"github.com/hashicorp/go-multierror"
)

func init() {
	RegisterConnector(&Connector{
		id:         "gitlab",
		config:     &GitlabFlags{},
		teamConfig: &GitlabTeamFlags{},
	})
}

type GitlabFlags struct {
	ClientID     string `long:"client-id" description:"(Required) Client id"`
	ClientSecret string `long:"client-secret" description:"(Required) Client secret"`
	Host         string `long:"host" description:"Hostname of Gitlab Enterprise deployment (Include scheme, No trailing slash)"`
}

func (self *GitlabFlags) Name() string {
	return "GitLab"
}

func (self *GitlabFlags) Validate() error {
	var errs *multierror.Error

	if self.ClientID == "" {
		errs = multierror.Append(errs, errors.New("Missing client-id"))
	}

	if self.ClientSecret == "" {
		errs = multierror.Append(errs, errors.New("Missing client-secret"))
	}

	return errs.ErrorOrNil()
}

func (self *GitlabFlags) Serialize(redirectURI string) ([]byte, error) {
	if err := self.Validate(); err != nil {
		return nil, err
	}

	return json.Marshal(gitlab.Config{
		ClientID:     self.ClientID,
		ClientSecret: self.ClientSecret,
		RedirectURI:  redirectURI,
		BaseURL:      self.Host,
	})
}

type GitlabTeamFlags struct {
	Users  []string `long:"user" description:"List of whitelisted GitLab users" value-name:"USERNAME"`
	Groups []string `long:"group" description:"List of whitelisted GitLab groups" value-name:"GROUP_NAME"`
}

func (self *GitlabTeamFlags) IsValid() bool {
	return len(self.Users) > 0 || len(self.Groups) > 0
}

func (self *GitlabTeamFlags) GetUsers() []string {
	return self.Users
}

func (self *GitlabTeamFlags) GetGroups() []string {
	return self.Groups
}

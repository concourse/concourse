package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/dex/connector/gitlab"
	multierror "github.com/hashicorp/go-multierror"
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

func (flag *GitlabFlags) Name() string {
	return "GitLab"
}

func (flag *GitlabFlags) Validate() error {
	var errs *multierror.Error

	if flag.ClientID == "" {
		errs = multierror.Append(errs, errors.New("Missing client-id"))
	}

	if flag.ClientSecret == "" {
		errs = multierror.Append(errs, errors.New("Missing client-secret"))
	}

	return errs.ErrorOrNil()
}

func (flag *GitlabFlags) Serialize(redirectURI string) ([]byte, error) {
	if err := flag.Validate(); err != nil {
		return nil, err
	}

	return json.Marshal(gitlab.Config{
		ClientID:     flag.ClientID,
		ClientSecret: flag.ClientSecret,
		RedirectURI:  redirectURI,
		BaseURL:      flag.Host,
	})
}

type GitlabTeamFlags struct {
	Users  []string `long:"user" description:"A whitelisted GitLab user" value-name:"USERNAME"`
	Groups []string `long:"group" description:"A whitelisted GitLab group" value-name:"GROUP_NAME"`
}

func (flag *GitlabTeamFlags) GetUsers() []string {
	return flag.Users
}

func (flag *GitlabTeamFlags) GetGroups() []string {
	return flag.Groups
}

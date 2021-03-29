package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/dex/connector/gitlab"
	multierror "github.com/hashicorp/go-multierror"
)

type GitlabFlags struct {
	ClientID     string `yaml:"client_id,omitempty"`
	ClientSecret string `yaml:"client_secret,omitempty"`
	Host         string `yaml:"host,omitempty"`
}

func (flag *GitlabFlags) ID() string {
	return GitlabConnectorID
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
	Users  []string `yaml:"users,omitempty" env:"CONCOURSE_MAIN_TEAM_GITLAB_USERS,CONCOURSE_MAIN_TEAM_GITLAB_USER" long:"user" description:"A whitelisted GitLab user" value-name:"USERNAME"`
	Groups []string `yaml:"groups,omitempty" env:"CONCOURSE_MAIN_TEAM_GITLAB_GROUPS,CONCOURSE_MAIN_TEAM_GITLAB_GROUP" long:"group" description:"A whitelisted GitLab group" value-name:"GROUP_NAME"`
}

func (flag *GitlabTeamFlags) ID() string {
	return GitlabConnectorID
}

func (flag *GitlabTeamFlags) GetUsers() []string {
	return flag.Users
}

func (flag *GitlabTeamFlags) GetGroups() []string {
	return flag.Groups
}

package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/dex/connector/gitea"
	multierror "github.com/hashicorp/go-multierror"
)

func init() {
	RegisterConnector(&Connector{
		id:         "gitea",
		config:     &GiteaFlags{},
		teamConfig: &GiteaTeamFlags{},
	})
}

type GiteaFlags struct {
	DisplayName  string `long:"display-name" description:"The auth provider name displayed to users on the login page (default: Gitea)"`
	ClientID     string `long:"client-id" description:"(Required) Client id"`
	ClientSecret string `long:"client-secret" description:"(Required) Client secret"`
	Host         string `long:"host" description:"Hostname of Gitea deployment (Include scheme, No trailing slash)"`
}

func (flag *GiteaFlags) Name() string {
	if flag.DisplayName == "" {
		return "Gitea"
	}
	return flag.DisplayName
}

func (flag *GiteaFlags) Validate() error {
	var errs *multierror.Error

	if flag.ClientID == "" {
		errs = multierror.Append(errs, errors.New("Missing client-id"))
	}

	if flag.ClientSecret == "" {
		errs = multierror.Append(errs, errors.New("Missing client-secret"))
	}

	return errs.ErrorOrNil()
}

func (flag *GiteaFlags) Serialize(redirectURI string) ([]byte, error) {
	if err := flag.Validate(); err != nil {
		return nil, err
	}

	return json.Marshal(gitea.Config{
		ClientID:      flag.ClientID,
		ClientSecret:  flag.ClientSecret,
		RedirectURI:   redirectURI,
		BaseURL:       flag.Host,
		LoadAllGroups: true,
	})
}

type GiteaTeamFlags struct {
	Users []string `long:"user" description:"An allowlisted Gitea user" value-name:"USERNAME"`
	Orgs  []string `long:"org" description:"An allowlisted Gitea org" value-name:"ORG_NAME"`
	Teams []string `long:"team" description:"An allowlisted Gitea team" value-name:"ORG_NAME:TEAM_NAME"`
}

func (flag *GiteaTeamFlags) GetUsers() []string {
	return flag.Users
}

func (flag *GiteaTeamFlags) GetGroups() []string {
	return append(flag.Orgs, flag.Teams...)
}

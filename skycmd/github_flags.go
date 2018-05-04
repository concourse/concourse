package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/coreos/dex/connector/github"
)

func init() {
	RegisterConnector(&Connector{
		id:          "github",
		displayName: "GitHub",
		config:      &GithubFlags{},
		teamConfig:  &GithubTeamFlags{},
	})
}

type GithubFlags struct {
	ClientID     string `long:"client-id" description:"Client id"`
	ClientSecret string `long:"client-secret" description:"Client secret"`
}

func (self *GithubFlags) IsValid() bool {
	return self.ClientID != "" && self.ClientSecret != ""
}

func (self *GithubFlags) Serialize(redirectURI string) ([]byte, error) {
	if !self.IsValid() {
		return nil, errors.New("Invalid config")
	}

	return json.Marshal(github.Config{
		ClientID:     self.ClientID,
		ClientSecret: self.ClientSecret,
		RedirectURI:  redirectURI,
	})
}

type GithubTeamFlags struct {
	Users  []string `json:"users" long:"user" description:"List of github users" value-name:"LOGIN"`
	Groups []string `json:"groups" long:"group" description:"List of github groups (e.g. my-org or my-org:my-team)" value-name:"ORG_NAME:TEAM_NAME"`
}

func (self *GithubTeamFlags) IsValid() bool {
	return len(self.Users) > 0 || len(self.Groups) > 0
}

func (self *GithubTeamFlags) GetUsers() []string {
	return self.Users
}

func (self *GithubTeamFlags) GetGroups() []string {
	return self.Groups
}

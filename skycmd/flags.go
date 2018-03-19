package skycmd

import (
	"strings"
	"time"

	"github.com/concourse/flag"
)

type AuthFlags struct {
	Expiration     time.Duration     `long:"expiration" default:"24h" description:"Length of time for which tokens are valid. Afterwards, users will have to log back in."`
	SigningKey     flag.PrivateKey   `long:"signing-key" description:"File containing an RSA private key, used to sign session tokens."`
	GithubFlags    GithubFlags       `group:"Github Auth" namespace:"github"`
	LocalUserFlags map[string]string `group:"Basic Auth" long:"local-basic-auth" value-name:"USERNAME:PASSWORD"`
}

type GithubFlags struct {
	ClientID     string `long:"client-id" description:"Github client id"`
	ClientSecret string `long:"client-secret" description:"Github client secret"`
}

func (self GithubFlags) IsValid() bool {
	return self.ClientID != "" && self.ClientSecret != ""
}

type GithubTeamFlags struct {
	Users  []string `json:"users" long:"user" description:"List of github users" value-name:"GITHUB_LOGIN"`
	Groups []string `json:"groups" long:"group" description:"List of github groups (e.g. my-org or my-org:my-team)" value-name:"GITHUB_ORG:GITHUB_TEAM"`
}

func (config GithubTeamFlags) IsValid() bool {
	return len(config.Users) > 0 || len(config.Groups) > 0
}

type LocalTeamFlags struct {
	Users []string `json:"users" long:"user" description:"List of basic auth users" value-name:"BASIC_AUTH_USERNAME"`
}

func (config LocalTeamFlags) IsValid() bool {
	return len(config.Users) > 0
}

type AuthTeamFlags struct {
	LocalTeamFlags  LocalTeamFlags  `group:"Basic Auth" namespace:"local"`
	GithubTeamFlags GithubTeamFlags `group:"Github Auth" namespace:"github"`
	NoAuth          bool            `long:"no-really-i-dont-want-any-auth" description:"Flag to disable any authorization"`
}

func (config AuthTeamFlags) IsValid() bool {
	return config.NoAuth || config.GithubTeamFlags.IsValid() || config.LocalTeamFlags.IsValid()
}

func (config AuthTeamFlags) Format() map[string][]string {
	users := []string{}
	groups := []string{}

	for _, user := range config.LocalTeamFlags.Users {
		users = append(users, "local:"+strings.ToLower(user))
	}

	for _, user := range config.GithubTeamFlags.Users {
		users = append(users, "github:"+strings.ToLower(user))
	}

	for _, group := range config.GithubTeamFlags.Groups {
		groups = append(groups, "github:"+strings.ToLower(group))
	}

	return map[string][]string{
		"users":  users,
		"groups": groups,
	}
}

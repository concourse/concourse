package skycmd

import (
	"strings"
	"time"

	"github.com/concourse/flag"
)

type AuthFlags struct {
	SecureCookies bool              `long:"secure-cookies" description:"Set secure flag on http cookies"`
	Expiration    time.Duration     `long:"expiration" default:"24h" description:"Length of time for which tokens are valid. Afterwards, users will have to log back in."`
	SigningKey    flag.PrivateKey   `long:"signing-key" description:"File containing an RSA private key, used to sign session tokens."`
	Github        GithubFlags       `group:"Github Auth" namespace:"github"`
	CF            CFFlags           `group:"CF Auth" namespace:"cf"`
	LDAP          LDAPFlags         `group:"LDAP Auth" namespace:"ldap"`
	LocalUsers    map[string]string `group:"Basic Auth" long:"local-basic-auth" value-name:"USERNAME:PASSWORD"`
}

type AuthTeamFlags struct {
	LocalTeamFlags  LocalTeamFlags  `group:"Basic Auth" namespace:"local"`
	GithubTeamFlags GithubTeamFlags `group:"Github Auth" namespace:"github"`
	CFTeamFlags     CFTeamFlags     `group:"CF Auth" namespace:"cf"`
	LDAPTeamFlags   LDAPTeamFlags   `group:"LDAP Auth" namespace:"ldap"`
	NoAuth          bool            `long:"no-really-i-dont-want-any-auth" description:"Flag to disable any authorization"`
}

func (config AuthTeamFlags) IsValid() bool {
	return config.NoAuth ||
		config.GithubTeamFlags.IsValid() ||
		config.LocalTeamFlags.IsValid() ||
		config.CFTeamFlags.IsValid() ||
		config.LDAPTeamFlags.IsValid()
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

	for _, user := range config.CFTeamFlags.Users {
		users = append(users, "cf:"+strings.ToLower(user))
	}

	for _, group := range config.CFTeamFlags.Groups {
		groups = append(groups, "cf:"+strings.ToLower(group))
	}

	for _, user := range config.LDAPTeamFlags.Users {
		users = append(users, "ldap:"+strings.ToLower(user))
	}

	for _, group := range config.LDAPTeamFlags.Groups {
		groups = append(groups, "ldap:"+strings.ToLower(group))
	}

	return map[string][]string{
		"users":  users,
		"groups": groups,
	}
}

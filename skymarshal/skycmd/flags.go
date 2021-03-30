package skycmd

import (
	"io/ioutil"
	"strings"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/flag"
)

const (
	BitbucketCloudConnectorID = "bitbucket-cloud"
	CFConnectorID             = "cf"
	GithubConnectorID         = "github"
	GitlabConnectorID         = "gitlab"
	LDAPConnectorID           = "ldap"
	MicrosoftConnectorID      = "microsoft"
	OAuthConnectorID          = "oauth"
	OIDCConnectorID           = "oidc"
	SAMLConnectorID           = "saml"
)

var ConnectorIDs = []string{
	BitbucketCloudConnectorID,
	CFConnectorID,
	GithubConnectorID,
	GitlabConnectorID,
	LDAPConnectorID,
	MicrosoftConnectorID,
	OAuthConnectorID,
	OIDCConnectorID,
	SAMLConnectorID,
}

type AuthFlags struct {
	SecureCookies bool              `yaml:"cookie_secure,omitempty"`
	Expiration    time.Duration     `yaml:"auth_duration,omitempty"`
	SigningKey    *flag.PrivateKey  `yaml:"session_signing_key,omitempty"`
	LocalUsers    map[string]string `yaml:"add_local_user,omitempty"`
	Clients       map[string]string `yaml:"add_client,omitempty"`

	Connectors ConnectorsConfig `yaml:"connectors,omitempty" ignore_env:"true"`
}

// XXX: IMPORTANT! Once fly has been converted to using cobra, all the go-flags
// tags can be removed from this AuthTeamFlags struct and any substructs of all
// the team connectors
type AuthTeamFlags struct {
	LocalUsers []string  `yaml:"local_user,omitempty" long:"local-user" description:"A whitelisted local concourse user. These are the users you've added at web startup with the --add-local-user flag." value-name:"USERNAME"`
	Config     flag.File `yaml:"config,omitempty" short:"c" long:"config" description:"Configuration file for specifying team params"`

	TeamConnectors TeamConnectorsConfig `yaml:"team_connectors,omitempty" ignore_env:"true"`
}

type ConnectorsConfig struct {
	BitbucketCloud *BitbucketCloudFlags `yaml:"bitbucket_cloud,omitempty"`
	CF             *CFFlags             `yaml:"cf,omitempty"`
	Github         *GithubFlags         `yaml:"github,omitempty"`
	Gitlab         *GitlabFlags         `yaml:"gitlab,omitempty"`
	LDAP           *LDAPFlags           `yaml:"ldap,omitempty"`
	Microsoft      *MicrosoftFlags      `yaml:"microsoft,omitempty"`
	OAuth          *OAuthFlags          `yaml:"oauth,omitempty"`
	OIDC           *OIDCFlags           `yaml:"oidc,omitempty"`
	SAML           *SAMLFlags           `yaml:"saml,omitempty"`
}

func (c ConnectorsConfig) ConfiguredConnectors() []Config {
	var configuredConnectors []Config
	if c.BitbucketCloud != nil {
		configuredConnectors = append(configuredConnectors, c.BitbucketCloud)
	}

	if c.CF != nil {
		configuredConnectors = append(configuredConnectors, c.CF)
	}

	if c.Github != nil {
		configuredConnectors = append(configuredConnectors, c.Github)
	}

	if c.Gitlab != nil {
		configuredConnectors = append(configuredConnectors, c.Gitlab)
	}

	if c.LDAP != nil {
		configuredConnectors = append(configuredConnectors, c.LDAP)
	}

	if c.Microsoft != nil {
		configuredConnectors = append(configuredConnectors, c.Microsoft)
	}

	if c.OAuth != nil {
		configuredConnectors = append(configuredConnectors, c.OAuth)
	}

	if c.OIDC != nil {
		configuredConnectors = append(configuredConnectors, c.OIDC)
	}

	if c.SAML != nil {
		configuredConnectors = append(configuredConnectors, c.SAML)
	}

	return configuredConnectors
}

type TeamConnectorsConfig struct {
	BitbucketCloud *BitbucketCloudTeamFlags `yaml:"bitbucket_cloud,omitempty" group:"Authentication (Bitbucket Cloud)" namespace:"bitbucket-cloud"`
	CF             *CFTeamFlags             `yaml:"cf,omitempty" group:"Authentication (CloudFoundry)" namespace:"cf"`
	Github         *GithubTeamFlags         `yaml:"github,omitempty" group:"Authentication (GitHub)" namespace:"github"`
	Gitlab         *GitlabTeamFlags         `yaml:"gitlab,omitempty" group:"Authentication (GitLab)" namespace:"gitlab"`
	LDAP           *LDAPTeamFlags           `yaml:"ldap,omitempty" group:"Authentication (LDAP)" namespace:"ldap"`
	Microsoft      *MicrosoftTeamFlags      `yaml:"microsoft,omitempty" group:"Authentication (Microsoft)" namespace:"microsoft"`
	OAuth          *OAuthTeamFlags          `yaml:"oauth,omitempty" group:"Authentication (OAuth2)" namespace:"oauth"`
	OIDC           *OIDCTeamFlags           `yaml:"oidc,omitempty" group:"Authentication (OIDC)" namespace:"oidc"`
	SAML           *SAMLTeamFlags           `yaml:"saml,omitempty" group:"Authentication (SAML)" namespace:"saml"`
}

func (c TeamConnectorsConfig) ConfiguredConnectors() []TeamConfig {
	var configuredConnectors []TeamConfig
	if c.BitbucketCloud != nil {
		configuredConnectors = append(configuredConnectors, c.BitbucketCloud)
	}

	if c.CF != nil {
		configuredConnectors = append(configuredConnectors, c.CF)
	}

	if c.Github != nil {
		configuredConnectors = append(configuredConnectors, c.Github)
	}

	if c.Gitlab != nil {
		configuredConnectors = append(configuredConnectors, c.Gitlab)
	}

	if c.LDAP != nil {
		configuredConnectors = append(configuredConnectors, c.LDAP)
	}

	if c.Microsoft != nil {
		configuredConnectors = append(configuredConnectors, c.Microsoft)
	}

	if c.OAuth != nil {
		configuredConnectors = append(configuredConnectors, c.OAuth)
	}

	if c.OIDC != nil {
		configuredConnectors = append(configuredConnectors, c.OIDC)
	}

	if c.SAML != nil {
		configuredConnectors = append(configuredConnectors, c.SAML)
	}

	return configuredConnectors
}

type Config interface {
	ID() string
	Name() string
	Serialize(redirectURI string) ([]byte, error)
}

type TeamConfig interface {
	ID() string
	GetUsers() []string
	GetGroups() []string
}

func (flag *AuthTeamFlags) Format() (atc.TeamAuth, error) {

	if path := flag.Config.Path(); path != "" {
		return flag.formatFromFile()

	}
	return flag.formatFromFlags()

}

// When formatting from a configuration file we iterate over each connector
// type and create a new instance of the TeamConfig object for each connector.
// These connectors all have their own unique configuration so we need to use
// mapstructure to decode the generic result into a known struct.

// e.g.
// The github connector has configuration for: users, teams, orgs
// The cf conncetor has configuration for: users, orgs, spaces

func (flag *AuthTeamFlags) formatFromFile() (atc.TeamAuth, error) {
	content, err := ioutil.ReadFile(flag.Config.Path())
	if err != nil {
		return nil, err
	}

	var data struct {
		Roles []struct {
			Name string `yaml:"name"`

			Local struct {
				Users []string `yaml:"users"`
			} `yaml:"local"`
			TeamConnectorsConfig
		} `yaml:"roles"`
	}

	if err = yaml.Unmarshal(content, &data); err != nil {
		return nil, err
	}

	auth := atc.TeamAuth{}

	for _, role := range data.Roles {
		users := []string{}
		groups := []string{}

		teamConfigs := role.TeamConnectorsConfig.ConfiguredConnectors()
		for _, teamConfig := range teamConfigs {
			for _, user := range teamConfig.GetUsers() {
				if user != "" {
					users = append(users, teamConfig.ID()+":"+strings.ToLower(user))
				}
			}

			for _, group := range teamConfig.GetGroups() {
				if group != "" {
					groups = append(groups, teamConfig.ID()+":"+strings.ToLower(group))
				}
			}
		}

		if role.Local.Users != nil {
			for _, user := range role.Local.Users {
				users = append(users, "local:"+strings.ToLower(user))
			}
		}

		if len(users) == 0 && len(groups) == 0 {
			continue
		}

		auth[role.Name] = map[string][]string{
			"users":  users,
			"groups": groups,
		}
	}

	if err := auth.Validate(); err != nil {
		return nil, err
	}

	return auth, nil
}

// When formatting team config from the command line flags, the connector's
// TeamConfig has already been populated by the flags library. All we need to
// do is grab the teamConfig object and extract the users and groups.

func (flag *AuthTeamFlags) formatFromFlags() (atc.TeamAuth, error) {

	users := []string{}
	groups := []string{}

	teamConfigs := flag.TeamConnectors.ConfiguredConnectors()
	for _, teamConfig := range teamConfigs {
		for _, user := range teamConfig.GetUsers() {
			if user != "" {
				users = append(users, teamConfig.ID()+":"+strings.ToLower(user))
			}
		}

		for _, group := range teamConfig.GetGroups() {
			if group != "" {
				groups = append(groups, teamConfig.ID()+":"+strings.ToLower(group))
			}
		}
	}

	for _, user := range flag.LocalUsers {
		if user != "" {
			users = append(users, "local:"+strings.ToLower(user))
		}
	}

	if len(users) == 0 && len(groups) == 0 {
		return nil, atc.ErrAuthConfigInvalid
	}

	return atc.TeamAuth{
		"owner": map[string][]string{
			"users":  users,
			"groups": groups,
		},
	}, nil
}

type skyDisplayUserIdGenerator struct {
	mapConnectorUserid map[string]string
}

func NewSkyDisplayUserIdGenerator(config map[string]string) atc.DisplayUserIdGenerator {
	return &skyDisplayUserIdGenerator{
		mapConnectorUserid: config,
	}
}

func (g *skyDisplayUserIdGenerator) DisplayUserId(connector, userid, username, preferredUsername, email string) string {
	if fieldName, ok := g.mapConnectorUserid[connector]; ok {
		switch fieldName {
		case "user_id":
			return userid
		case "name":
			return username
		case "username":
			return preferredUsername
		case "email":
			return email
		}
	}

	// For unconfigured connector, applies a default rule.
	if preferredUsername != "" {
		return preferredUsername
	} else if userid != "" {
		return userid
	} else {
		return username
	}
}

package skycmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/concourse/flag"
	"github.com/jessevdk/go-flags"
)

type Config interface {
	Serialize(redirectURI string) ([]byte, error)
}

type TeamConfig interface {
	IsValid() bool
	GetUsers() []string
	GetGroups() []string
}

type Connector struct {
	id          string
	displayName string
	config      Config
	teamConfig  TeamConfig
}

func (self *Connector) ID() string {
	return self.id
}

func (self *Connector) Name() string {
	return self.displayName
}

func (self *Connector) Config(issuer string) ([]byte, error) {
	return self.config.Serialize(issuer)
}

func (self *Connector) HasTeamConfig() bool {
	return self.teamConfig.IsValid()
}

func (self *Connector) GetTeamUsers() []string {
	return self.teamConfig.GetUsers()
}

func (self *Connector) GetTeamGroups() []string {
	return self.teamConfig.GetGroups()
}

var connectors []*Connector

func RegisterConnector(connector *Connector) {
	connectors = append(connectors, connector)
}

func GetConnectors() []*Connector {
	return connectors
}

func WireConnectors(group *flags.Group) {
	for _, c := range connectors {
		description := fmt.Sprintf("%s (%s)", group.ShortDescription, c.Name())
		connGroup, _ := group.AddGroup(description, description, c.config)
		connGroup.Namespace = c.ID()
	}
}

func WireTeamConnectors(group *flags.Group) {
	for _, c := range connectors {
		description := fmt.Sprintf("%s (%s)", group.ShortDescription, c.Name())
		connTeamGroup, _ := group.AddGroup(description, description, c.teamConfig)
		connTeamGroup.Namespace = c.ID()
	}
}

type AuthFlags struct {
	SecureCookies bool              `long:"secure-cookies" description:"Set secure flag on http cookies"`
	Expiration    time.Duration     `long:"expiration" default:"24h" description:"Length of time for which tokens are valid. Afterwards, users will have to log back in."`
	SigningKey    flag.PrivateKey   `long:"signing-key" description:"File containing an RSA private key, used to sign auth tokens."`
	LocalUsers    map[string]string `group:"Authentication (Local Users)" long:"add-local-user" value-name:"USERNAME:PASSWORD"`
}

type AuthTeamFlags struct {
	LocalUsers []string `json:"users" long:"local-user" description:"List of local concourse users" value-name:"USERNAME"`
	NoAuth     bool     `long:"no-really-i-dont-want-any-auth" description:"Flag to disable any authorization"`
}

func (self *AuthTeamFlags) Format() (map[string][]string, error) {
	users := []string{}
	groups := []string{}

	for _, connector := range connectors {

		if !connector.HasTeamConfig() {
			continue
		}

		for _, user := range connector.GetTeamUsers() {
			users = append(users, connector.ID()+":"+strings.ToLower(user))
		}

		for _, group := range connector.GetTeamGroups() {
			groups = append(groups, connector.ID()+":"+strings.ToLower(group))
		}
	}

	for _, user := range self.LocalUsers {
		users = append(users, "local:"+strings.ToLower(user))
	}

	if len(users) == 0 && len(groups) == 0 && !self.NoAuth {
		return nil, errors.New("Must configure auth")
	}

	return map[string][]string{
		"users":  users,
		"groups": groups,
	}, nil
}

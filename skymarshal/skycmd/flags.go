package skycmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/concourse/flag"
	flags "github.com/jessevdk/go-flags"
)

var connectors []*Connector

const DefaultAuthRole string = "owner"

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
	SecureCookies bool              `long:"cookie-secure" description:"Force sending secure flag on http cookies"`
	Expiration    time.Duration     `long:"auth-duration" default:"24h" description:"Length of time for which tokens are valid. Afterwards, users will have to log back in."`
	SigningKey    *flag.PrivateKey  `long:"session-signing-key" description:"File containing an RSA private key, used to sign auth tokens."`
	LocalUsers    map[string]string `long:"add-local-user" description:"List of username:password combinations for all your local users. The password can be bcrypted - if so, it must have a minimum cost of 10." value-name:"USERNAME:PASSWORD"`
}

type AuthTeamFlags struct {
	LocalUsers    []string `long:"local-user" description:"List of whitelisted local concourse users. These are the users you've added at atc startup with the --add-local-user flag." value-name:"USERNAME"`
	AllowAllUsers bool     `long:"allow-all-users" description:"Setting this flag will whitelist all logged in users in the system. ALL OF THEM. If, for example, you've configured GitHub, any user with a GitHub account will have access to your team."`
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

	if len(users) == 0 && len(groups) == 0 && !self.AllowAllUsers {
		return nil, errors.New("No auth methods have been configured.")
	}

	return map[string][]string{
		"users":  users,
		"groups": groups,
	}, nil
}

type Config interface {
	Name() string
	Serialize(redirectURI string) ([]byte, error)
}

type TeamConfig interface {
	IsValid() bool
	GetUsers() []string
	GetGroups() []string
}

type Connector struct {
	id         string
	config     Config
	teamConfig TeamConfig
}

func (self *Connector) ID() string {
	return self.id
}

func (self *Connector) Name() string {
	return self.config.Name()
}

func (self *Connector) Serialize(redirectURI string) ([]byte, error) {
	return self.config.Serialize(redirectURI)
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

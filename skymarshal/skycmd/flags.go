package skycmd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"
	"time"

	"github.com/concourse/flag"
	flags "github.com/jessevdk/go-flags"
	"github.com/mitchellh/mapstructure"
	yaml "gopkg.in/yaml.v2"
)

var ErrRequireAllowAllUsersFlag = errors.New("ErrRequireAllowAllUsersFlag")
var ErrRequireAllowAllUsersConfig = errors.New("ErrRequireAllowAllUsersConfig")

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
	SecureCookies bool              `long:"cookie-secure" description:"Force sending secure flag on http cookies"`
	Expiration    time.Duration     `long:"auth-duration" default:"24h" description:"Length of time for which tokens are valid. Afterwards, users will have to log back in."`
	SigningKey    *flag.PrivateKey  `long:"session-signing-key" description:"File containing an RSA private key, used to sign auth tokens."`
	LocalUsers    map[string]string `long:"add-local-user" description:"List of username:password combinations for all your local users. The password can be bcrypted - if so, it must have a minimum cost of 10." value-name:"USERNAME:PASSWORD"`
}

type AuthTeamFlags struct {
	LocalUsers    []string  `long:"local-user" description:"List of whitelisted local concourse users. These are the users you've added at atc startup with the --add-local-user flag." value-name:"USERNAME"`
	AllowAllUsers bool      `long:"allow-all-users" description:"Setting this flag will whitelist all logged in users in the system. ALL OF THEM. If, for example, you've configured GitHub, any user with a GitHub account will have access to your team."`
	Config        flag.File `short:"c" long:"config" description:"Configuration file for specifying team params"`
}

func (self *AuthTeamFlags) Format() (AuthConfig, error) {

	if path := self.Config.Path(); path != "" {
		return self.formatFromConfig()

	} else {
		return self.formatFromFlags()
	}
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

// We use some reflect magic to zero out our teamConfig
// before we parse each new role.
func (self *Connector) Parse(config interface{}) error {

	typeof := reflect.TypeOf(self.teamConfig)
	if typeof.Kind() == reflect.Ptr {
		typeof = typeof.Elem()
	}

	valueof := reflect.ValueOf(self.teamConfig)
	if valueof.Kind() == reflect.Ptr {
		valueof = valueof.Elem()
	}

	instance := reflect.New(typeof).Elem()
	if valueof.CanSet() {
		valueof.Set(instance)
	}

	return mapstructure.Decode(config, &self.teamConfig)
}

func (self *AuthTeamFlags) formatFromConfig() (AuthConfig, error) {

	content, err := ioutil.ReadFile(self.Config.Path())
	if err != nil {
		return nil, err
	}

	var data struct {
		Roles []map[string]interface{} `yaml:"roles"`
	}
	if err = yaml.Unmarshal(content, &data); err != nil {
		return nil, err
	}

	auth := AuthConfig{}

	for _, role := range data.Roles {
		roleName := role["name"].(string)

		self.AllowAllUsers, _ = role["allow_all_users"].(bool)

		users := []string{}
		groups := []string{}

		for _, connector := range connectors {
			config, ok := role[connector.ID()]
			if ok {
				connector.Parse(config)
			}

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

		if conf, ok := role["local"].(map[interface{}]interface{}); ok {
			for _, user := range conf["users"].([]interface{}) {
				users = append(users, "local:"+strings.ToLower(user.(string)))
			}
		}
		if len(users) == 0 && len(groups) == 0 && !self.AllowAllUsers {
			return nil, ErrRequireAllowAllUsersConfig
		}

		auth[roleName] = map[string][]string{
			"users":  users,
			"groups": groups,
		}
	}

	return auth, nil
}

func (self *AuthTeamFlags) formatFromFlags() (AuthConfig, error) {

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
		return nil, ErrRequireAllowAllUsersFlag
	}

	return AuthConfig{
		"owner": map[string][]string{
			"users":  users,
			"groups": groups,
		},
	}, nil
}

type AuthConfig map[string]map[string][]string

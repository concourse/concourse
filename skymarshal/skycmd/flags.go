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
	"sigs.k8s.io/yaml"
)

var ErrAuthNotConfiguredFromFlags = errors.New("ErrAuthNotConfiguredFromFlags")
var ErrAuthNotConfiguredFromFile = errors.New("ErrAuthNotConfiguredFromFile")

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
	Clients       map[string]string `long:"add-client" description:"List of client_id:client_secret combinations" value-name:"CLIENT_ID:CLIENT_SECRET"`
}

type AuthTeamFlags struct {
	LocalUsers []string  `long:"local-user" description:"A whitelisted local concourse user. These are the users you've added at web startup with the --add-local-user flag." value-name:"USERNAME"`
	Config     flag.File `short:"c" long:"config" description:"Configuration file for specifying team params"`
}

func (flag *AuthTeamFlags) Format() (AuthConfig, error) {

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

func (flag *AuthTeamFlags) formatFromFile() (AuthConfig, error) {

	content, err := ioutil.ReadFile(flag.Config.Path())
	if err != nil {
		return nil, err
	}

	var data struct {
		Roles []map[string]interface{} `json:"roles"`
	}
	if err = yaml.Unmarshal(content, &data); err != nil {
		return nil, err
	}

	auth := AuthConfig{}

	for _, role := range data.Roles {
		roleName := role["name"].(string)

		users := []string{}
		groups := []string{}

		for _, connector := range connectors {
			config, ok := role[connector.ID()]
			if !ok {
				continue
			}

			teamConfig, err := connector.newTeamConfig()
			if err != nil {
				return nil, err
			}

			err = mapstructure.Decode(config, &teamConfig)
			if err != nil {
				return nil, err
			}

			for _, user := range teamConfig.GetUsers() {
				if user != "" {
					users = append(users, connector.ID()+":"+strings.ToLower(user))
				}
			}

			for _, group := range teamConfig.GetGroups() {
				if group != "" {
					groups = append(groups, connector.ID()+":"+strings.ToLower(group))
				}
			}
		}

		if conf, ok := role["local"].(map[string]interface{}); ok {
			for _, user := range conf["users"].([]interface{}) {
				if user != "" {
					users = append(users, "local:"+strings.ToLower(user.(string)))
				}
			}
		}

		if len(users) == 0 && len(groups) == 0 {
			continue
		}

		auth[roleName] = map[string][]string{
			"users":  users,
			"groups": groups,
		}
	}

	return auth, nil
}

// When formatting team config from the command line flags, the connector's
// TeamConfig has already been populated by the flags library. All we need to
// do is grab the teamConfig object and extract the users and groups.

func (flag *AuthTeamFlags) formatFromFlags() (AuthConfig, error) {

	users := []string{}
	groups := []string{}

	for _, connector := range connectors {

		teamConfig := connector.teamConfig

		for _, user := range teamConfig.GetUsers() {
			if user != "" {
				users = append(users, connector.ID()+":"+strings.ToLower(user))
			}
		}

		for _, group := range teamConfig.GetGroups() {
			if group != "" {
				groups = append(groups, connector.ID()+":"+strings.ToLower(group))
			}
		}
	}

	for _, user := range flag.LocalUsers {
		if user != "" {
			users = append(users, "local:"+strings.ToLower(user))
		}
	}

	if len(users) == 0 && len(groups) == 0 {
		return nil, ErrAuthNotConfiguredFromFlags
	}

	return AuthConfig{
		"owner": map[string][]string{
			"users":  users,
			"groups": groups,
		},
	}, nil
}

type Config interface {
	Name() string
	Serialize(redirectURI string) ([]byte, error)
}

type TeamConfig interface {
	GetUsers() []string
	GetGroups() []string
}

type Connector struct {
	id         string
	config     Config
	teamConfig TeamConfig
}

func (con *Connector) ID() string {
	return con.id
}

func (con *Connector) Name() string {
	return con.config.Name()
}

func (con *Connector) Serialize(redirectURI string) ([]byte, error) {
	return con.config.Serialize(redirectURI)
}

func (con *Connector) newTeamConfig() (TeamConfig, error) {

	typeof := reflect.TypeOf(con.teamConfig)
	if typeof.Kind() == reflect.Ptr {
		typeof = typeof.Elem()
	}

	valueof := reflect.ValueOf(con.teamConfig)
	if valueof.Kind() == reflect.Ptr {
		valueof = valueof.Elem()
	}

	instance := reflect.New(typeof).Interface()
	res, ok := instance.(TeamConfig)
	if !ok {
		return nil, errors.New("Invalid TeamConfig")
	}

	return res, nil
}

type AuthConfig map[string]map[string][]string

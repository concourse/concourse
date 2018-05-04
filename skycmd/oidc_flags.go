package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/flag"
	"github.com/coreos/dex/connector/oidc"
)

func init() {
	RegisterConnector(&Connector{
		id:          "oidc",
		displayName: "OIDC",
		config:      &OIDCFlags{},
		teamConfig:  &OIDCTeamFlags{},
	})
}

type OIDCFlags struct {
	Issuer             string      `long:"issuer" description:"Issuer URL"`
	ClientID           string      `long:"client-id" description:"Client id"`
	ClientSecret       string      `long:"client-secret" description:"Client secret"`
	Scopes             []string    `long:"scope" description:"Requested scope"`
	GroupsKey          string      `long:"groups-key" description:"Groups Key"`
	RootCAs            []flag.File `long:"root-ca" description:"Root CA"`
	InsecureSkipVerify bool        `long:"skip-ssl-validation" description:"Skip SSL validation"`
}

func (self *OIDCFlags) IsValid() bool {
	return self.Issuer != "" && self.ClientID != "" && self.ClientSecret != ""
}

func (self *OIDCFlags) Serialize(redirectURI string) ([]byte, error) {
	if !self.IsValid() {
		return nil, errors.New("Not configured")
	}

	rootCAs := []string{}
	for _, file := range self.RootCAs {
		rootCAs = append(rootCAs, file.Path())
	}

	return json.Marshal(oidc.Config{
		Issuer:             self.Issuer,
		ClientID:           self.ClientID,
		ClientSecret:       self.ClientSecret,
		Scopes:             self.Scopes,
		GroupsKey:          self.GroupsKey,
		RootCAs:            rootCAs,
		InsecureSkipVerify: self.InsecureSkipVerify,
		RedirectURI:        redirectURI,
	})
}

type OIDCTeamFlags struct {
	Users  []string `json:"users" long:"user" description:"List of OIDC users" value-name:"OIDC_USERNAME"`
	Groups []string `json:"groups" long:"group" description:"List of OIDC groups" value-name:"OIDC_GROUP"`
}

func (self *OIDCTeamFlags) IsValid() bool {
	return len(self.Users) > 0 || len(self.Groups) > 0
}

func (self *OIDCTeamFlags) GetUsers() []string {
	return self.Users
}

func (self *OIDCTeamFlags) GetGroups() []string {
	return self.Groups
}

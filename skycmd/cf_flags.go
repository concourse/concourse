package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/flag"
	"github.com/coreos/dex/connector/cf"
)

func init() {
	RegisterConnector(&Connector{
		id:          "cf",
		displayName: "CF",
		config:      &CFFlags{},
		teamConfig:  &CFTeamFlags{},
	})
}

type CFFlags struct {
	ClientID           string      `long:"client-id" description:"Client id"`
	ClientSecret       string      `long:"client-secret" description:"Client secret"`
	APIURL             string      `long:"api-url" description:"API URL"`
	RootCAs            []flag.File `long:"root-ca" description:"Root CA"`
	InsecureSkipVerify bool        `long:"skip-ssl-validation" description:"Skip SSL validation"`
}

func (self *CFFlags) IsValid() bool {
	return self.ClientID != "" && self.ClientSecret != "" && self.APIURL != ""
}

func (self *CFFlags) Serialize(redirectURI string) ([]byte, error) {
	if !self.IsValid() {
		return nil, errors.New("Invalid config")
	}

	rootCAs := []string{}
	for _, file := range self.RootCAs {
		rootCAs = append(rootCAs, file.Path())
	}

	return json.Marshal(cf.Config{
		ClientID:           self.ClientID,
		ClientSecret:       self.ClientSecret,
		APIURL:             self.APIURL,
		RootCAs:            rootCAs,
		InsecureSkipVerify: self.InsecureSkipVerify,
		RedirectURI:        redirectURI,
	})
}

type CFTeamFlags struct {
	Users  []string `json:"users" long:"user" description:"List of cf users" value-name:"USERNAME"`
	Groups []string `json:"groups" long:"group" description:"List of cf groups (e.g. my-org or my-org:my-space)" value-name:"ORG_NAME:SPACE_NAME"`
}

func (self *CFTeamFlags) IsValid() bool {
	return len(self.Users) > 0 || len(self.Groups) > 0
}

func (self *CFTeamFlags) GetUsers() []string {
	return self.Users
}

func (self *CFTeamFlags) GetGroups() []string {
	return self.Groups
}

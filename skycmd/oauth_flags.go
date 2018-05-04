package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/flag"
	"github.com/coreos/dex/connector/oauth"
)

func init() {
	RegisterConnector(&Connector{
		id:          "oauth",
		displayName: "OAuth",
		config:      &OAuthFlags{},
		teamConfig:  &OAuthTeamFlags{},
	})
}

type OAuthFlags struct {
	ClientID           string      `long:"client-id" description:"Client id"`
	ClientSecret       string      `long:"client-secret" description:"Client secret"`
	AuthURL            string      `long:"auth-url" description:"Authorization URL"`
	TokenURL           string      `long:"token-url" description:"Token URL"`
	UserInfoURL        string      `long:"userinfo-url" description:"UserInfo URL"`
	Scopes             []string    `long:"scope" description:"Requested scope"`
	GroupsKey          string      `long:"groups-key" description:"Groups Key"`
	RootCAs            []flag.File `long:"root-ca" description:"Root CA"`
	InsecureSkipVerify bool        `long:"skip-ssl-validation" description:"Skip SSL validation"`
}

func (self *OAuthFlags) IsValid() bool {
	return self.AuthURL != "" &&
		self.TokenURL != "" &&
		self.UserInfoURL != "" &&
		self.ClientID != "" &&
		self.ClientSecret != ""
}

func (self *OAuthFlags) Serialize(redirectURI string) ([]byte, error) {
	if !self.IsValid() {
		return nil, errors.New("Not configured")
	}

	rootCAs := []string{}
	for _, file := range self.RootCAs {
		rootCAs = append(rootCAs, file.Path())
	}

	return json.Marshal(oauth.Config{
		ClientID:           self.ClientID,
		ClientSecret:       self.ClientSecret,
		AuthorizationURL:   self.AuthURL,
		TokenURL:           self.TokenURL,
		UserInfoURL:        self.UserInfoURL,
		Scopes:             self.Scopes,
		GroupsKey:          self.GroupsKey,
		RootCAs:            rootCAs,
		InsecureSkipVerify: self.InsecureSkipVerify,
		RedirectURI:        redirectURI,
	})
}

type OAuthTeamFlags struct {
	Users  []string `json:"users" long:"user" description:"List of OAuth users" value-name:"OAUTH_USERNAME"`
	Groups []string `json:"groups" long:"group" description:"List of OAuth groups" value-name:"OAUTH_GROUP"`
}

func (self *OAuthTeamFlags) IsValid() bool {
	return len(self.Users) > 0 || len(self.Groups) > 0
}

func (self *OAuthTeamFlags) GetUsers() []string {
	return self.Users
}

func (self *OAuthTeamFlags) GetGroups() []string {
	return self.Groups
}

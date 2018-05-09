package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/flag"
	"github.com/coreos/dex/connector/oidc"
	"github.com/hashicorp/go-multierror"
)

func init() {
	RegisterConnector(&Connector{
		id:         "oidc",
		config:     &OIDCFlags{},
		teamConfig: &OIDCTeamFlags{},
	})
}

type OIDCFlags struct {
	DisplayName        string      `long:"display-name" description:"Display Name"`
	Issuer             string      `long:"issuer" description:"Issuer URL"`
	ClientID           string      `long:"client-id" description:"Client id"`
	ClientSecret       string      `long:"client-secret" description:"Client secret"`
	Scopes             []string    `long:"scope" description:"Requested scope"`
	GroupsKey          string      `long:"groups-key" description:"Groups Key"`
	RootCAs            []flag.File `long:"root-ca" description:"Root CA"`
	InsecureSkipVerify bool        `long:"skip-ssl-validation" description:"Skip SSL validation"`
}

func (self *OIDCFlags) Name() string {
	if self.DisplayName != "" {
		return self.DisplayName
	} else {
		return "OIDC"
	}
}

func (self *OIDCFlags) Validate() error {
	var errs *multierror.Error

	if self.Issuer == "" {
		errs = multierror.Append(errs, errors.New("Missing issuer"))
	}

	if self.ClientID == "" {
		errs = multierror.Append(errs, errors.New("Missing client-id"))
	}

	if self.ClientSecret == "" {
		errs = multierror.Append(errs, errors.New("Missing client-secret"))
	}

	return errs.ErrorOrNil()
}

func (self *OIDCFlags) Serialize(redirectURI string) ([]byte, error) {
	if err := self.Validate(); err != nil {
		return nil, err
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
	Users  []string `json:"users" long:"user" description:"List of whitelisted OIDC users" value-name:"USERNAME"`
	Groups []string `json:"groups" long:"group" description:"List of whitelisted OIDC groups" value-name:"GROUP_NAME"`
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

package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/dex/connector/oidc"
	"github.com/concourse/flag"
	multierror "github.com/hashicorp/go-multierror"
)

func init() {
	RegisterConnector(&Connector{
		id:         "oidc",
		config:     &OIDCFlags{},
		teamConfig: &OIDCTeamFlags{},
	})
}

type OIDCFlags struct {
	DisplayName        string      `long:"display-name" description:"The auth provider name displayed to users on the login page"`
	Issuer             string      `long:"issuer" description:"(Required) An OIDC issuer URL that will be used to discover provider configuration using the .well-known/openid-configuration"`
	ClientID           string      `long:"client-id" description:"(Required) Client id"`
	ClientSecret       string      `long:"client-secret" description:"(Required) Client secret"`
	Scopes             []string    `long:"scope" description:"Any additional scopes that need to be requested during authorization"`
	GroupsKey          string      `long:"groups-key" description:"The groups key indicates which claim to use to map external groups to Concourse teams."`
	HostedDomains      []string    `long:"hosted-domains" description:"List of whitelisted domains when using Google, only users from a listed domain will be allowed to log in"`
	CACerts            []flag.File `long:"ca-cert" description:"CA Certificate"`
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

	caCerts := []string{}
	for _, file := range self.CACerts {
		caCerts = append(caCerts, file.Path())
	}

	return json.Marshal(oidc.Config{
		Issuer:             self.Issuer,
		ClientID:           self.ClientID,
		ClientSecret:       self.ClientSecret,
		Scopes:             self.Scopes,
		GroupsKey:          self.GroupsKey,
		HostedDomains:      self.HostedDomains,
		RootCAs:            caCerts,
		InsecureSkipVerify: self.InsecureSkipVerify,
		RedirectURI:        redirectURI,
	})
}

type OIDCTeamFlags struct {
	Users  []string `json:"users" long:"user" description:"List of whitelisted OIDC users" value-name:"USERNAME"`
	Groups []string `json:"groups" long:"group" description:"List of whitelisted OIDC groups" value-name:"GROUP_NAME"`
}

func (self *OIDCTeamFlags) GetUsers() []string {
	return self.Users
}

func (self *OIDCTeamFlags) GetGroups() []string {
	return self.Groups
}

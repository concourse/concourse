package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/flag"
	"github.com/coreos/dex/connector/cf"
	"github.com/hashicorp/go-multierror"
)

func init() {
	RegisterConnector(&Connector{
		id:         "cf",
		config:     &CFFlags{},
		teamConfig: &CFTeamFlags{},
	})
}

type CFFlags struct {
	ClientID           string      `long:"client-id" description:"(Required) Client id"`
	ClientSecret       string      `long:"client-secret" description:"(Required) Client secret"`
	APIURL             string      `long:"api-url" description:"(Required) The base API URL of your CF deployment. It will use this information to discover information about the authentication provider."`
	CACerts            []flag.File `long:"ca-cert" description:"CA Certificate"`
	InsecureSkipVerify bool        `long:"skip-ssl-validation" description:"Skip SSL validation"`
}

func (self *CFFlags) Name() string {
	return "CloudFoundry"
}

func (self *CFFlags) Validate() error {
	var errs *multierror.Error

	if self.APIURL == "" {
		errs = multierror.Append(errs, errors.New("Missing api-url"))
	}

	if self.ClientID == "" {
		errs = multierror.Append(errs, errors.New("Missing client-id"))
	}

	if self.ClientSecret == "" {
		errs = multierror.Append(errs, errors.New("Missing client-secret"))
	}

	return errs.ErrorOrNil()
}

func (self *CFFlags) Serialize(redirectURI string) ([]byte, error) {
	if err := self.Validate(); err != nil {
		return nil, err
	}

	caCerts := []string{}
	for _, file := range self.CACerts {
		caCerts = append(caCerts, file.Path())
	}

	return json.Marshal(cf.Config{
		ClientID:           self.ClientID,
		ClientSecret:       self.ClientSecret,
		APIURL:             self.APIURL,
		RootCAs:            caCerts,
		InsecureSkipVerify: self.InsecureSkipVerify,
		RedirectURI:        redirectURI,
	})
}

type CFTeamFlags struct {
	Users      []string `long:"user" description:"List of whitelisted CloudFoundry users." value-name:"USERNAME"`
	Orgs       []string `long:"org" description:"List of whitelisted CloudFoundry orgs" value-name:"ORG_NAME"`
	Spaces     []string `long:"space" description:"List of whitelisted CloudFoundry spaces" value-name:"ORG_NAME:SPACE_NAME"`
	SpaceGuids []string `long:"space-guid" description:"(Deprecated) List of whitelisted CloudFoundry space guids" value-name:"SPACE_GUID"`
}

func (self *CFTeamFlags) IsValid() bool {
	return len(self.Users) > 0 || len(self.Orgs) > 0 || len(self.Spaces) > 0 || len(self.SpaceGuids) > 0
}

func (self *CFTeamFlags) GetUsers() []string {
	return self.Users
}

func (self *CFTeamFlags) GetGroups() []string {
	var groups []string
	groups = append(groups, self.Orgs...)
	groups = append(groups, self.Spaces...)
	groups = append(groups, self.SpaceGuids...)
	return groups
}

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
	ClientID           string      `long:"client-id" description:"Client id"`
	ClientSecret       string      `long:"client-secret" description:"Client secret"`
	APIURL             string      `long:"api-url" description:"API URL"`
	RootCAs            []flag.File `long:"root-ca" description:"Root CA"`
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
	Users  []string `long:"user" description:"List of whitelisted CloudFoundry users." value-name:"USERNAME"`
	Orgs   []string `long:"org" description:"List of whitelisted CloudFoundry orgs" value-name:"ORG_NAME"`
	Spaces []string `long:"space" description:"List of whitelisted CloudFoundry spaces" value-name:"ORG_NAME:SPACE_NAME"`
}

func (self *CFTeamFlags) IsValid() bool {
	return len(self.Users) > 0 || len(self.Orgs) > 0 || len(self.Spaces) > 0
}

func (self *CFTeamFlags) GetUsers() []string {
	return self.Users
}

func (self *CFTeamFlags) GetGroups() []string {
	return append(self.Orgs, self.Spaces...)
}

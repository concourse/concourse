package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/dex/connector/cf"
	"github.com/concourse/flag"
	multierror "github.com/hashicorp/go-multierror"
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

func (flag *CFFlags) Name() string {
	return "CloudFoundry"
}

func (flag *CFFlags) Validate() error {
	var errs *multierror.Error

	if flag.APIURL == "" {
		errs = multierror.Append(errs, errors.New("Missing api-url"))
	}

	if flag.ClientID == "" {
		errs = multierror.Append(errs, errors.New("Missing client-id"))
	}

	if flag.ClientSecret == "" {
		errs = multierror.Append(errs, errors.New("Missing client-secret"))
	}

	return errs.ErrorOrNil()
}

func (flag *CFFlags) Serialize(redirectURI string) ([]byte, error) {
	if err := flag.Validate(); err != nil {
		return nil, err
	}

	caCerts := []string{}
	for _, file := range flag.CACerts {
		caCerts = append(caCerts, file.Path())
	}

	return json.Marshal(cf.Config{
		ClientID:           flag.ClientID,
		ClientSecret:       flag.ClientSecret,
		APIURL:             flag.APIURL,
		RootCAs:            caCerts,
		InsecureSkipVerify: flag.InsecureSkipVerify,
		RedirectURI:        redirectURI,
	})
}

type CFTeamFlags struct {
	Users      []string `long:"user" description:"List of whitelisted CloudFoundry users." value-name:"USERNAME"`
	Orgs       []string `long:"org" description:"List of whitelisted CloudFoundry orgs" value-name:"ORG_NAME"`
	Spaces     []string `long:"space" description:"List of whitelisted CloudFoundry spaces" value-name:"ORG_NAME:SPACE_NAME"`
	SpaceGuids []string `long:"space-guid" description:"(Deprecated) List of whitelisted CloudFoundry space guids" value-name:"SPACE_GUID"`
}

func (flag *CFTeamFlags) GetUsers() []string {
	return flag.Users
}

func (flag *CFTeamFlags) GetGroups() []string {
	var groups []string
	groups = append(groups, flag.Orgs...)
	groups = append(groups, flag.Spaces...)
	groups = append(groups, flag.SpaceGuids...)
	return groups
}

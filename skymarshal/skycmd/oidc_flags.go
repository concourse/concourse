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
	GroupsKey          string      `long:"groups-key" default:"groups" description:"The groups key indicates which claim to use to map external groups to Concourse teams."`
	UserNameKey        string      `long:"user-name-key" default:"username" description:"The user name key indicates which claim to use to map an external user name to a Concourse user name."`
	HostedDomains      []string    `long:"hosted-domains" description:"List of whitelisted domains when using Google, only users from a listed domain will be allowed to log in"`
	CACerts            []flag.File `long:"ca-cert" description:"CA Certificate"`
	InsecureSkipVerify bool        `long:"skip-ssl-validation" description:"Skip SSL validation"`
}

func (flag *OIDCFlags) Name() string {
	if flag.DisplayName != "" {
		return flag.DisplayName
	}
	return "OIDC"
}

func (flag *OIDCFlags) Validate() error {
	var errs *multierror.Error

	if flag.Issuer == "" {
		errs = multierror.Append(errs, errors.New("Missing issuer"))
	}

	if flag.ClientID == "" {
		errs = multierror.Append(errs, errors.New("Missing client-id"))
	}

	if flag.ClientSecret == "" {
		errs = multierror.Append(errs, errors.New("Missing client-secret"))
	}

	return errs.ErrorOrNil()
}

func (flag *OIDCFlags) Serialize(redirectURI string) ([]byte, error) {
	if err := flag.Validate(); err != nil {
		return nil, err
	}

	caCerts := []string{}
	for _, file := range flag.CACerts {
		caCerts = append(caCerts, file.Path())
	}

	return json.Marshal(oidc.Config{
		Issuer:             flag.Issuer,
		ClientID:           flag.ClientID,
		ClientSecret:       flag.ClientSecret,
		Scopes:             flag.Scopes,
		GroupsKey:          flag.GroupsKey,
		UserNameKey:        flag.UserNameKey,
		HostedDomains:      flag.HostedDomains,
		RootCAs:            caCerts,
		InsecureSkipVerify: flag.InsecureSkipVerify,
		RedirectURI:        redirectURI,
	})
}

type OIDCTeamFlags struct {
	Users  []string `json:"users" long:"user" description:"A whitelisted OIDC user" value-name:"USERNAME"`
	Groups []string `json:"groups" long:"group" description:"A whitelisted OIDC group" value-name:"GROUP_NAME"`
}

func (flag *OIDCTeamFlags) GetUsers() []string {
	return flag.Users
}

func (flag *OIDCTeamFlags) GetGroups() []string {
	return flag.Groups
}

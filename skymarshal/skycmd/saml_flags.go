package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/concourse/flag"
	"github.com/concourse/dex/connector/saml"
	multierror "github.com/hashicorp/go-multierror"
)

type SAMLFlags struct {
	SsoURL             string    `yaml:"sso_url,omitempty"`
	CACert             flag.File `yaml:"ca_cert,omitempty"`
	EntityIssuer       string    `yaml:"entity_issuer,omitempty"`
	SsoIssuer          string    `yaml:"sso_issuer,omitempty"`
	UsernameAttr       string    `yaml:"username_attr,omitempty"`
	EmailAttr          string    `yaml:"email_attr,omitempty"`
	GroupsAttr         string    `yaml:"groups_attr,omitempty"`
	GroupsDelim        string    `yaml:"groups_delim,omitempty"`
	NameIDPolicyFormat string    `yaml:"name_id_policy_format,omitempty"`
	InsecureSkipVerify bool      `yaml:"skip_ssl_validation,omitempty"`
}

func (flag *SAMLFlags) Validate() error {
	var errs *multierror.Error

	if flag.SsoURL == "" {
		errs = multierror.Append(errs, errors.New("Missing sso-url"))
	}

	if flag.CACert == "" {
		errs = multierror.Append(errs, errors.New("Missing ca-cert"))
	}

	return errs.ErrorOrNil()
}

func (flag *SAMLFlags) Serialize(redirectURI string) ([]byte, error) {
	if err := flag.Validate(); err != nil {
		return nil, err
	}

	return json.Marshal(saml.Config{
		SSOURL:                          flag.SsoURL,
		CA:                              flag.CACert.Path(),
		EntityIssuer:                    flag.EntityIssuer,
		SSOIssuer:                       flag.SsoIssuer,
		InsecureSkipSignatureValidation: flag.InsecureSkipVerify,
		UsernameAttr:                    flag.UsernameAttr,
		EmailAttr:                       flag.EmailAttr,
		GroupsAttr:                      flag.GroupsAttr,
		GroupsDelim:                     flag.GroupsDelim,
		NameIDPolicyFormat:              flag.NameIDPolicyFormat,
		RedirectURI:                     redirectURI,
	})
}

type SAMLTeamFlags struct {
	Users  []string `yaml:"users" env:"CONCOURSE_MAIN_TEAM_SAML_USERS,CONCOURSE_MAIN_TEAM_SAML_USER" json:"users" long:"user" description:"A whitelisted SAML user" value-name:"USERNAME"`
	Groups []string `yaml:"groups" env:"CONCOURSE_MAIN_TEAM_SAML_GROUPS,CONCOURSE_MAIN_TEAM_SAML_GROUP" json:"groups" long:"group" description:"A whitelisted SAML group" value-name:"GROUP_NAME"`
}

func (flag *SAMLTeamFlags) ID() string {
	return SAMLConnectorID
}

func (flag *SAMLTeamFlags) GetUsers() []string {
	return flag.Users
}

func (flag *SAMLTeamFlags) GetGroups() []string {
	return flag.Groups
}

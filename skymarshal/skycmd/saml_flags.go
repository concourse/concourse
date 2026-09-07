package skycmd

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/concourse/concourse/flag"
	"github.com/concourse/dex/connector/saml"
	multierror "github.com/hashicorp/go-multierror"
)

func init() {
	RegisterConnector(&Connector{
		id:         "saml",
		config:     &SAMLFlags{},
		teamConfig: &SAMLTeamFlags{},
	})
}

type SAMLFlags struct {
	DisplayName        string    `long:"display-name" description:"The auth provider name displayed to users on the login page"`
	SsoURL             string    `long:"sso-url" description:"(Required) SSO URL used for POST value"`
	CACert             flag.File `long:"ca-cert" description:"(Required) CA Certificate"`
	EntityIssuer       string    `long:"entity-issuer" description:"Manually specify dex's Issuer value."`
	SsoIssuer          string    `long:"sso-issuer" description:"Issuer value expected in the SAML response."`
	UsernameAttr       string    `long:"username-attr" default:"name" description:"The user name indicates which claim to use to map an external user name to a Concourse user name."`
	EmailAttr          string    `long:"email-attr" default:"email" description:"The email indicates which claim to use to map an external user email to a Concourse user email."`
	GroupsAttr         string    `long:"groups-attr" default:"groups" description:"The groups key indicates which attribute to use to map external groups to Concourse teams."`
	GroupsDelim        string    `long:"groups-delim" description:"If specified, groups are returned as string, this delimiter will be used to split the group string."`
	NameIDPolicyFormat string    `long:"name-id-policy-format" description:"Requested format of the NameID. The NameID value is mapped to the ID Token 'sub' claim."`
	InsecureSkipVerify bool      `long:"skip-ssl-validation" description:"Skip SSL validation"`
}

func (flag *SAMLFlags) Name() string {
	if flag.DisplayName != "" {
		return flag.DisplayName
	}
	return "SAML"
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
	Users  []string `json:"users" long:"user" description:"A whitelisted SAML user" value-name:"USERNAME"`
	Groups []string `json:"groups" long:"group" description:"A whitelisted SAML group. Wrap a group in double quotes to keep it from being split on commas when set via environment variable." value-name:"GROUP_NAME"`
}

func (flag *SAMLTeamFlags) GetUsers() []string {
	return flag.Users
}

func (flag *SAMLTeamFlags) GetGroups() []string {
	return rejoinQuotedGroups(flag.Groups)
}

// Environment values for slice flags are split on commas before they reach
// this struct, which breaks LDAP-style group DNs such as
// 'CN=admins,OU=Groups,DC=example,DC=com'. A group wrapped in double quotes
// is reassembled from the split pieces and the quotes are stripped, so
// CONCOURSE_MAIN_TEAM_SAML_GROUP='"CN=admins,OU=Groups,DC=example,DC=com"'
// yields a single group. Unquoted values keep the existing comma-split
// behavior.
func rejoinQuotedGroups(values []string) []string {
	var groups []string

	for i := 0; i < len(values); i++ {
		value := values[i]
		if !strings.HasPrefix(value, `"`) {
			groups = append(groups, value)
			continue
		}

		joined := value
		last := i
		for (len(joined) < 2 || !strings.HasSuffix(joined, `"`)) && last+1 < len(values) {
			last++
			joined += "," + values[last]
		}

		if len(joined) >= 2 && strings.HasSuffix(joined, `"`) {
			groups = append(groups, joined[1:len(joined)-1])
			i = last
		} else {
			// no closing quote; leave the value untouched
			groups = append(groups, value)
		}
	}

	return groups
}

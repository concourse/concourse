package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/dex/connector/ldap"
	"github.com/concourse/flag"
	multierror "github.com/hashicorp/go-multierror"
)

func init() {
	RegisterConnector(&Connector{
		id:         "ldap",
		config:     &LDAPFlags{},
		teamConfig: &LDAPTeamFlags{},
	})
}

type LDAPFlags struct {
	DisplayName        string    `long:"display-name" description:"The auth provider name displayed to users on the login page"`
	Host               string    `long:"host" description:"(Required) The host and optional port of the LDAP server. If port isn't supplied, it will be guessed based on the TLS configuration. 389 or 636."`
	BindDN             string    `long:"bind-dn" description:"(Required) Bind DN for searching LDAP users and groups. Typically this is a read-only user."`
	BindPW             string    `long:"bind-pw" description:"(Required) Bind Password for the user specified by 'bind-dn'"`
	InsecureNoSSL      bool      `long:"insecure-no-ssl" description:"Required if LDAP host does not use TLS."`
	InsecureSkipVerify bool      `long:"insecure-skip-verify" description:"Skip certificate verification"`
	StartTLS           bool      `long:"start-tls" description:"Start on insecure port, then negotiate TLS"`
	CACert             flag.File `long:"ca-cert" description:"CA certificate"`

	UserSearch struct {
		BaseDN    string `long:"user-search-base-dn" description:"BaseDN to start the search from. For example 'cn=users,dc=example,dc=com'"`
		Filter    string `long:"user-search-filter" description:"Optional filter to apply when searching the directory. For example '(objectClass=person)'"`
		Username  string `long:"user-search-username" description:"Attribute to match against the inputted username. This will be translated and combined with the other filter as '(<attr>=<username>)'."`
		Scope     string `long:"user-search-scope" description:"Can either be: 'sub' - search the whole sub tree or 'one' - only search one level. Defaults to 'sub'."`
		IDAttr    string `long:"user-search-id-attr" description:"A mapping of attributes on the user entry to claims. Defaults to 'uid'."`
		EmailAttr string `long:"user-search-email-attr" description:"A mapping of attributes on the user entry to claims. Defaults to 'mail'."`
		NameAttr  string `long:"user-search-name-attr" description:"A mapping of attributes on the user entry to claims."`
	}

	GroupSearch struct {
		BaseDN    string `long:"group-search-base-dn" description:"BaseDN to start the search from. For example 'cn=groups,dc=example,dc=com'"`
		Filter    string `long:"group-search-filter" description:"Optional filter to apply when searching the directory. For example '(objectClass=posixGroup)'"`
		Scope     string `long:"group-search-scope" description:"Can either be: 'sub' - search the whole sub tree or 'one' - only search one level. Defaults to 'sub'."`
		UserAttr  string `long:"group-search-user-attr" description:"Adds an additional requirement to the filter that an attribute in the group match the user's attribute value. The exact filter being added is: (<groupAttr>=<userAttr value>)"`
		GroupAttr string `long:"group-search-group-attr" description:"Adds an additional requirement to the filter that an attribute in the group match the user's attribute value. The exact filter being added is: (<groupAttr>=<userAttr value>)"`
		NameAttr  string `long:"group-search-name-attr" description:"The attribute of the group that represents its name."`
	}
}

func (self *LDAPFlags) Name() string {
	if self.DisplayName != "" {
		return self.DisplayName
	} else {
		return "LDAP"
	}
}

func (self *LDAPFlags) Validate() error {
	var errs *multierror.Error

	if self.Host == "" {
		errs = multierror.Append(errs, errors.New("Missing host"))
	}

	if self.BindDN == "" {
		errs = multierror.Append(errs, errors.New("Missing bind-dn"))
	}

	if self.BindPW == "" {
		errs = multierror.Append(errs, errors.New("Missing bind-pw"))
	}

	return errs.ErrorOrNil()
}

func (self *LDAPFlags) Serialize(redirectURI string) ([]byte, error) {
	if err := self.Validate(); err != nil {
		return nil, err
	}

	ldapConfig := ldap.Config{
		Host:               self.Host,
		BindDN:             self.BindDN,
		BindPW:             self.BindPW,
		InsecureNoSSL:      self.InsecureNoSSL,
		InsecureSkipVerify: self.InsecureSkipVerify,
		StartTLS:           self.StartTLS,
		RootCA:             self.CACert.Path(),
	}

	ldapConfig.UserSearch.BaseDN = self.UserSearch.BaseDN
	ldapConfig.UserSearch.Filter = self.UserSearch.Filter
	ldapConfig.UserSearch.Username = self.UserSearch.Username
	ldapConfig.UserSearch.Scope = self.UserSearch.Scope
	ldapConfig.UserSearch.IDAttr = self.UserSearch.IDAttr
	ldapConfig.UserSearch.EmailAttr = self.UserSearch.EmailAttr
	ldapConfig.UserSearch.NameAttr = self.UserSearch.NameAttr

	ldapConfig.GroupSearch.BaseDN = self.GroupSearch.BaseDN
	ldapConfig.GroupSearch.Filter = self.GroupSearch.Filter
	ldapConfig.GroupSearch.Scope = self.GroupSearch.Scope
	ldapConfig.GroupSearch.UserAttr = self.GroupSearch.UserAttr
	ldapConfig.GroupSearch.GroupAttr = self.GroupSearch.GroupAttr
	ldapConfig.GroupSearch.NameAttr = self.GroupSearch.NameAttr

	return json.Marshal(ldapConfig)
}

type LDAPTeamFlags struct {
	Users  []string `json:"users" long:"user" description:"List of whitelisted LDAP users" value-name:"USERNAME"`
	Groups []string `json:"groups" long:"group" description:"List of whitelisted LDAP groups" value-name:"GROUP_NAME"`
}

func (self *LDAPTeamFlags) GetUsers() []string {
	return self.Users
}

func (self *LDAPTeamFlags) GetGroups() []string {
	return self.Groups
}

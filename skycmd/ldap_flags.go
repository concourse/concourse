package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/flag"
	"github.com/coreos/dex/connector/ldap"
	"github.com/hashicorp/go-multierror"
)

func init() {
	RegisterConnector(&Connector{
		id:         "ldap",
		config:     &LDAPFlags{},
		teamConfig: &LDAPTeamFlags{},
	})
}

type LDAPFlags struct {
	DisplayName        string    `long:"display-name" description:"Display Name"`
	Host               string    `long:"host" description:"Host"`
	BindDN             string    `long:"bind-dn" description:"Bind DN"`
	BindPW             string    `long:"bind-pw" description:"Bind PW"`
	InsecureNoSSL      bool      `long:"insecure-no-ssl" description:"Don't use ssl"`
	InsecureSkipVerify bool      `long:"insecure-skip-verify" description:"Skip certificate verification"`
	StartTLS           bool      `long:"start-tls" description:"Start on insecure port, then negotiate TLS"`
	CACert             flag.File `long:"ca-cert" description:"CA certificate"`

	UserSearch struct {
		BaseDN    string `long:"user-search-base-dn" description:"Base DN"`
		Filter    string `long:"user-search-filter" description:"Filter"`
		Username  string `long:"user-search-username" description:"Username"`
		Scope     string `long:"user-search-scope" description:"Scope"`
		IDAttr    string `long:"user-search-id-attr" description:"ID Attr"`
		EmailAttr string `long:"user-search-email-attr" description:"Email Attr"`
		NameAttr  string `long:"user-search-name-attr" description:"Name Attr"`
	}

	GroupSearch struct {
		BaseDN    string `long:"group-search-base-dn" description:"Base DN"`
		Filter    string `long:"group-search-filter" description:"Filter"`
		Scope     string `long:"group-search-scope" description:"Scope"`
		UserAttr  string `long:"group-search-user-attr" description:"User Attr"`
		GroupAttr string `long:"group-search-group-attr" description:"Group Attr"`
		NameAttr  string `long:"group-search-name-attr" description:"Name Attr"`
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

func (self *LDAPTeamFlags) IsValid() bool {
	return len(self.Users) > 0 || len(self.Groups) > 0
}

func (self *LDAPTeamFlags) GetUsers() []string {
	return self.Users
}

func (self *LDAPTeamFlags) GetGroups() []string {
	return self.Groups
}

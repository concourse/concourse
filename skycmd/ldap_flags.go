package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/flag"
	"github.com/coreos/dex/connector/ldap"
)

func init() {
	RegisterConnector(&Connector{
		id:          "ldap",
		displayName: "LDAP",
		config:      &LDAPFlags{},
		teamConfig:  &LDAPTeamFlags{},
	})
}

type LDAPFlags struct {
	Host               string    `long:"host" description:"Host"`
	BindDN             string    `long:"bind-dn" description:"Bind DN"`
	BindPW             string    `long:"bind-pw" description:"Bind PW"`
	InsecureNoSSL      bool      `long:"insecure-no-ssl" description:"Don't use ssl"`
	InsecureSkipVerify bool      `long:"insecure-skip-verify" description:"Skip certificate verification"`
	StartTLS           bool      `long:"start-tls" description:"Start on insecure port, then negotiate TLS"`
	RootCA             flag.File `long:"root-ca" description:"Root CA certificate"`

	UserSearch struct {
		BaseDN    string `long:"base-dn" description:"Base DN"`
		Filter    string `long:"filter" description:"Filter"`
		Username  string `long:"username" description:"Username"`
		Scope     string `long:"scope" description:"Scope"`
		IDAttr    string `long:"id-attr" description:"ID Attr"`
		EmailAttr string `long:"email-attr" description:"Email Attr"`
		NameAttr  string `long:"name-attr" description:"Name Attr"`
	} `group:"User Search" namespace:"user-search"`

	GroupSearch struct {
		BaseDN    string `long:"base-dn" description:"Base DN"`
		Filter    string `long:"filter" description:"Filter"`
		Scope     string `long:"scope" description:"Scope"`
		UserAttr  string `long:"user-attr" description:"User Attr"`
		GroupAttr string `long:"group-attr" description:"Group Attr"`
		NameAttr  string `long:"name-attr" description:"Name Attr"`
	} `group:"Group Search" namespace:"group-search"`
}

func (self *LDAPFlags) IsValid() bool {
	return self.Host != "" && self.BindDN != "" && self.BindPW != ""
}

func (self *LDAPFlags) Serialize(redirectURI string) ([]byte, error) {
	if !self.IsValid() {
		return nil, errors.New("Not configured")
	}

	ldapConfig := ldap.Config{
		Host:               self.Host,
		BindDN:             self.BindDN,
		BindPW:             self.BindPW,
		InsecureNoSSL:      self.InsecureNoSSL,
		InsecureSkipVerify: self.InsecureSkipVerify,
		StartTLS:           self.StartTLS,
		RootCA:             self.RootCA.Path(),
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
	Users  []string `json:"users" long:"user" description:"List of ldap users" value-name:"USERNAME"`
	Groups []string `json:"groups" long:"group" description:"List of ldap groups" value-name:"GROUP NAME"`
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

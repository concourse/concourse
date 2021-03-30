package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/flag"
	"github.com/concourse/dex/connector/ldap"
	multierror "github.com/hashicorp/go-multierror"
)

type LDAPFlags struct {
	DisplayName        string    `yaml:"display_name,omitempty"`
	Host               string    `yaml:"host,omitempty"`
	BindDN             string    `yaml:"bind_dn,omitempty"`
	BindPW             string    `yaml:"bind_pw,omitempty"`
	InsecureNoSSL      bool      `yaml:"insecure_no_ssl,omitempty"`
	InsecureSkipVerify bool      `yaml:"insecure_skip_verify,omitempty"`
	StartTLS           bool      `yaml:"start_tls,omitempty"`
	CACert             flag.File `yaml:"ca_cert,omitempty"`

	UserSearch UserSearchConfig `yaml:"user_search,omitempty"`

	GroupSearch GroupSearchConfig `yaml:"group_search,omitempty"`
}

type UserSearchConfig struct {
	BaseDN    string `yaml:"base_dn,omitempty"`
	Filter    string `yaml:"filter,omitempty"`
	Username  string `yaml:"username,omitempty"`
	Scope     string `yaml:"scope,omitempty"`
	IDAttr    string `yaml:"id_attr,omitempty"`
	EmailAttr string `yaml:"email_attr,omitempty"`
	NameAttr  string `yaml:"name_attr,omitempty"`
}

type GroupSearchConfig struct {
	BaseDN    string `yaml:"base_dn,omitempty"`
	Filter    string `yaml:"filter,omitempty"`
	Scope     string `yaml:"scope,omitempty"`
	UserAttr  string `yaml:"user_attr,omitempty"`
	GroupAttr string `yaml:"group_attr,omitempty"`
	NameAttr  string `yaml:"name_attr,omitempty"`
}

func (flag *LDAPFlags) ID() string {
	return LDAPConnectorID
}

func (flag *LDAPFlags) Name() string {
	if flag.DisplayName != "" {
		return flag.DisplayName
	}
	return "LDAP"
}

func (flag *LDAPFlags) Validate() error {
	var errs *multierror.Error

	if flag.Host == "" {
		errs = multierror.Append(errs, errors.New("Missing host"))
	}

	if flag.BindDN == "" {
		errs = multierror.Append(errs, errors.New("Missing bind-dn"))
	}

	if flag.BindPW == "" {
		errs = multierror.Append(errs, errors.New("Missing bind-pw"))
	}

	return errs.ErrorOrNil()
}

func (flag *LDAPFlags) Serialize(redirectURI string) ([]byte, error) {
	if err := flag.Validate(); err != nil {
		return nil, err
	}

	ldapConfig := ldap.Config{
		Host:               flag.Host,
		BindDN:             flag.BindDN,
		BindPW:             flag.BindPW,
		InsecureNoSSL:      flag.InsecureNoSSL,
		InsecureSkipVerify: flag.InsecureSkipVerify,
		StartTLS:           flag.StartTLS,
		RootCA:             flag.CACert.Path(),
	}

	ldapConfig.UserSearch.BaseDN = flag.UserSearch.BaseDN
	ldapConfig.UserSearch.Filter = flag.UserSearch.Filter
	ldapConfig.UserSearch.Username = flag.UserSearch.Username
	ldapConfig.UserSearch.Scope = flag.UserSearch.Scope
	ldapConfig.UserSearch.IDAttr = flag.UserSearch.IDAttr
	ldapConfig.UserSearch.EmailAttr = flag.UserSearch.EmailAttr
	ldapConfig.UserSearch.NameAttr = flag.UserSearch.NameAttr

	ldapConfig.GroupSearch.BaseDN = flag.GroupSearch.BaseDN
	ldapConfig.GroupSearch.Filter = flag.GroupSearch.Filter
	ldapConfig.GroupSearch.Scope = flag.GroupSearch.Scope
	ldapConfig.GroupSearch.UserAttr = flag.GroupSearch.UserAttr
	ldapConfig.GroupSearch.GroupAttr = flag.GroupSearch.GroupAttr
	ldapConfig.GroupSearch.NameAttr = flag.GroupSearch.NameAttr

	return json.Marshal(ldapConfig)
}

type LDAPTeamFlags struct {
	Users  []string `yaml:"users,omitempty" env:"CONCOURSE_MAIN_TEAM_LDAP_USERS,CONCOURSE_MAIN_TEAM_LDAP_USER" json:"users" long:"user" description:"A whitelisted LDAP user" value-name:"USERNAME"`
	Groups []string `yaml:"groups,omitempty" env:"CONCOURSE_MAIN_TEAM_LDAP_GROUPS,CONCOURSE_MAIN_TEAM_LDAP_GROUP" json:"groups" long:"group" description:"A whitelisted LDAP group" value-name:"GROUP_NAME"`
}

func (flag *LDAPTeamFlags) ID() string {
	return LDAPConnectorID
}

func (flag *LDAPTeamFlags) GetUsers() []string {
	return flag.Users
}

func (flag *LDAPTeamFlags) GetGroups() []string {
	return flag.Groups
}

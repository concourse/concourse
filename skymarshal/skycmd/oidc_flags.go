package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/dex/connector/oidc"
	"github.com/concourse/flag"
	"github.com/hashicorp/go-multierror"
)

const OIDCConnectorID = "oidc"

type OIDCFlags struct {
	Enabled                   bool       `yaml:"enabled,omitempty"`
	DisplayName               string     `yaml:"display_name,omitempty"`
	Issuer                    string     `yaml:"issuer,omitempty"`
	ClientID                  string     `yaml:"client_id,omitempty"`
	ClientSecret              string     `yaml:"client_secret,omitempty"`
	Scopes                    []string   `yaml:"scope,omitempty"`
	GroupsKey                 string     `yaml:"groups_key,omitempty"`
	UserNameKey               string     `yaml:"user_name_key,omitempty"`
	HostedDomains             []string   `yaml:"hosted_domains,omitempty"`
	CACerts                   flag.Files `yaml:"ca_cert,omitempty"`
	InsecureSkipVerify        bool       `yaml:"skip_ssl_validation,omitempty"`
	DisableGroups             bool       `yaml:"disable_groups,omitempty"`
	InsecureSkipEmailVerified bool       `yaml:"skip_email_verified_validation,omitempty"`
}

func (flag *OIDCFlags) ID() string {
	return OIDCConnectorID
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

	config := oidc.Config{
		Issuer:                    flag.Issuer,
		ClientID:                  flag.ClientID,
		ClientSecret:              flag.ClientSecret,
		Scopes:                    flag.Scopes,
		UserNameKey:               flag.UserNameKey,
		HostedDomains:             flag.HostedDomains,
		RootCAs:                   caCerts,
		InsecureSkipVerify:        flag.InsecureSkipVerify,
		RedirectURI:               redirectURI,
		InsecureEnableGroups:      !flag.DisableGroups,
		InsecureSkipEmailVerified: flag.InsecureSkipEmailVerified,
	}

	config.ClaimMapping.GroupsKey = flag.GroupsKey
	config.ClaimMapping.PreferredUsernameKey = flag.UserNameKey

	return json.Marshal(config)
}

type OIDCTeamFlags struct {
	Users  []string `yaml:"users,omitempty" env:"CONCOURSE_MAIN_TEAM_OIDC_USERS,CONCOURSE_MAIN_TEAM_OIDC_USER" json:"users" long:"user" description:"A whitelisted OIDC user" value-name:"USERNAME"`
	Groups []string `yaml:"groups,omitempty" env:"CONCOURSE_MAIN_TEAM_OIDC_GROUPS,CONCOURSE_MAIN_TEAM_OIDC_GROUP" json:"groups" long:"group" description:"A whitelisted OIDC group" value-name:"GROUP_NAME"`
}

func (flag *OIDCTeamFlags) ID() string {
	return OIDCConnectorID
}

func (flag *OIDCTeamFlags) GetUsers() []string {
	return flag.Users
}

func (flag *OIDCTeamFlags) GetGroups() []string {
	return flag.Groups
}

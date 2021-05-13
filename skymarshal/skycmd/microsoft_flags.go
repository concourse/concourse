package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/dex/connector/microsoft"
	multierror "github.com/hashicorp/go-multierror"
)

const MicrosoftConnectorID = "microsoft"

type MicrosoftFlags struct {
	Enabled            bool     `yaml:"enabled,omitempty"`
	ClientID           string   `yaml:"client_id,omitempty"`
	ClientSecret       string   `yaml:"client_secret,omitempty"`
	Tenant             string   `yaml:"tenant,omitempty"`
	Groups             []string `yaml:"groups,omitempty"`
	OnlySecurityGroups bool     `yaml:"only_security_groups,omitempty"`
}

func (flag *MicrosoftFlags) ID() string {
	return MicrosoftConnectorID
}

func (flag *MicrosoftFlags) Name() string {
	return "Microsoft"
}

func (flag *MicrosoftFlags) Validate() error {
	var errs *multierror.Error

	if flag.ClientID == "" {
		errs = multierror.Append(errs, errors.New("Missing client-id"))
	}

	if flag.ClientSecret == "" {
		errs = multierror.Append(errs, errors.New("Missing client-secret"))
	}

	return errs.ErrorOrNil()
}

func (flag *MicrosoftFlags) Serialize(redirectURI string) ([]byte, error) {
	if err := flag.Validate(); err != nil {
		return nil, err
	}

	return json.Marshal(microsoft.Config{
		ClientID:           flag.ClientID,
		ClientSecret:       flag.ClientSecret,
		RedirectURI:        redirectURI,
		Tenant:             flag.Tenant,
		Groups:             flag.Groups,
		OnlySecurityGroups: flag.OnlySecurityGroups,
	})
}

type MicrosoftTeamFlags struct {
	Users  []string `yaml:"users,omitempty" env:"CONCOURSE_MAIN_TEAM_MICROSOFT_USERS,CONCOURSE_MAIN_TEAM_MICROSOFT_USER" long:"user" description:"A whitelisted Microsoft user" value-name:"USERNAME"`
	Groups []string `yaml:"groups,omitempty" env:"CONCOURSE_MAIN_TEAM_MICROSOFT_GROUPS,CONCOURSE_MAIN_TEAM_MICROSOFT_GROUP" long:"group" description:"A whitelisted Microsoft group" value-name:"GROUP_NAME"`
}

func (flag *MicrosoftTeamFlags) ID() string {
	return MicrosoftConnectorID
}

func (flag *MicrosoftTeamFlags) GetUsers() []string {
	return flag.Users
}

func (flag *MicrosoftTeamFlags) GetGroups() []string {
	return flag.Groups
}

package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/dex/connector/microsoft"
	multierror "github.com/hashicorp/go-multierror"
)

func init() {
	RegisterConnector(&Connector{
		id:         "microsoft",
		config:     &MicrosoftFlags{},
		teamConfig: &MicrosoftTeamFlags{},
	})
}

type MicrosoftFlags struct {
	ClientID           string   `long:"client-id" description:"(Required) Client id"`
	ClientSecret       string   `long:"client-secret" description:"(Required) Client secret"`
	Tenant             string   `long:"tenant" description:"Microsoft Tenant limitation (common, consumers, organizations, tenant name or tenant uuid)"`
	Groups             []string `long:"groups" description:"Allowed Active Directory Groups"`
	OnlySecurityGroups bool     `long:"only-security-groups" description:"Only fetch security groups"`
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
	Users  []string `long:"user" description:"A whitelisted Microsoft user" value-name:"USERNAME"`
	Groups []string `long:"group" description:"A whitelisted Microsoft group" value-name:"GROUP_NAME"`
}

func (flag *MicrosoftTeamFlags) GetUsers() []string {
	return flag.Users
}

func (flag *MicrosoftTeamFlags) GetGroups() []string {
	return flag.Groups
}

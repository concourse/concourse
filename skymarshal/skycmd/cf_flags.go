package skycmd

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/concourse/dex/connector/cf"
	"github.com/concourse/flag"
	multierror "github.com/hashicorp/go-multierror"
)

const CFConnectorID = "cf"

type CFFlags struct {
	Enabled            bool       `yaml:"enabled,omitempty"`
	ClientID           string     `yaml:"client_id,omitempty"`
	ClientSecret       string     `yaml:"client_secret,omitempty"`
	APIURL             string     `yaml:"api_url,omitempty"`
	CACerts            flag.Files `yaml:"ca_cert,omitempty"`
	InsecureSkipVerify bool       `yaml:"skip_ssl_validation,omitempty"`
}

func (flag *CFFlags) ID() string {
	return CFConnectorID
}

func (flag *CFFlags) Name() string {
	return "CloudFoundry"
}

func (flag *CFFlags) Validate() error {
	var errs *multierror.Error

	if flag.APIURL == "" {
		errs = multierror.Append(errs, errors.New("Missing api-url"))
	}

	if flag.ClientID == "" {
		errs = multierror.Append(errs, errors.New("Missing client-id"))
	}

	if flag.ClientSecret == "" {
		errs = multierror.Append(errs, errors.New("Missing client-secret"))
	}

	return errs.ErrorOrNil()
}

func (flag *CFFlags) Serialize(redirectURI string) ([]byte, error) {
	if err := flag.Validate(); err != nil {
		return nil, err
	}

	caCerts := []string{}
	for _, file := range flag.CACerts {
		caCerts = append(caCerts, file.Path())
	}

	return json.Marshal(cf.Config{
		ClientID:           flag.ClientID,
		ClientSecret:       flag.ClientSecret,
		APIURL:             flag.APIURL,
		RootCAs:            caCerts,
		InsecureSkipVerify: flag.InsecureSkipVerify,
		RedirectURI:        redirectURI,
	})
}

type CFTeamFlags struct {
	Users            []string `yaml:"users,omitempty" env:"CONCOURSE_MAIN_TEAM_CF_USERS,CONCOURSE_MAIN_TEAM_CF_USER" long:"user" description:"A whitelisted CloudFoundry user" value-name:"USERNAME"`
	Orgs             []string `yaml:"orgs,omitempty" env:"CONCOURSE_MAIN_TEAM_CF_ORGS,CONCOURSE_MAIN_TEAM_CF_ORG" long:"org" description:"A whitelisted CloudFoundry org" value-name:"ORG_NAME"`
	Spaces           []string `yaml:"spaces,omitempty" env:"CONCOURSE_MAIN_TEAM_CF_SPACES,CONCOURSE_MAIN_TEAM_CF_SPACE" long:"space" description:"(Deprecated) A whitelisted CloudFoundry space for users with the 'developer' role" value-name:"ORG_NAME:SPACE_NAME"`
	SpacesAll        []string `yaml:"spaces_with_any_role,omitempty" env:"CONCOURSE_MAIN_TEAM_CF_SPACES_WITH_ANY_ROLE,CONCOURSE_MAIN_TEAM_CF_SPACE_WITH_ANY_ROLE" long:"space-with-any-role" description:"A whitelisted CloudFoundry space for users with any role" value-name:"ORG_NAME:SPACE_NAME"`
	SpacesDeveloper  []string `yaml:"spaces_with_developer_role,omitempty" env:"CONCOURSE_MAIN_TEAM_CF_SPACES_WITH_DEVELOPER_ROLE,CONCOURSE_MAIN_TEAM_CF_SPACE_WITH_DEVELOPER_ROLE" long:"space-with-developer-role" description:"A whitelisted CloudFoundry space for users with the 'developer' role" value-name:"ORG_NAME:SPACE_NAME"`
	SpacesAuditor    []string `yaml:"spaces_with_auditor_role,omitempty" env:"CONCOURSE_MAIN_TEAM_CF_SPACES_WITH_AUDITOR_ROLE,CONCOURSE_MAIN_TEAM_CF_SPACE_WITH_AUDITOR_ROLE" long:"space-with-auditor-role" description:"A whitelisted CloudFoundry space for users with the 'auditor' role" value-name:"ORG_NAME:SPACE_NAME"`
	SpacesManager    []string `yaml:"spaces_with_manager_role,omitempty" env:"CONCOURSE_MAIN_TEAM_CF_SPACES_WITH_MANAGER_ROLE,CONCOURSE_MAIN_TEAM_CF_SPACE_WITH_MANAGER_ROLE" long:"space-with-manager-role" description:"A whitelisted CloudFoundry space for users with the 'manager' role" value-name:"ORG_NAME:SPACE_NAME"`
	SpaceGuids       []string `yaml:"space_guids,omitempty" env:"CONCOURSE_MAIN_TEAM_CF_SPACE_GUIDS,CONCOURSE_MAIN_TEAM_CF_SPACE_GUID" long:"space-guid" description:"A whitelisted CloudFoundry space guid" value-name:"SPACE_GUID"`
	SpaceGuidsLegacy []string `yaml:"spaceguids,omitempty"`
}

func (flag *CFTeamFlags) ID() string {
	return CFConnectorID
}

func (flag *CFTeamFlags) GetUsers() []string {
	return flag.Users
}

func (flag *CFTeamFlags) GetGroups() []string {
	var groups []string
	groups = append(groups, flag.Orgs...)
	groups = append(groups, flag.SpacesAll...)
	for _, space := range flag.Spaces {
		groups = append(groups, fmt.Sprintf("%s:developer", space))
	}
	for _, space := range flag.SpacesDeveloper {
		groups = append(groups, fmt.Sprintf("%s:developer", space))
	}
	for _, space := range flag.SpacesAuditor {
		groups = append(groups, fmt.Sprintf("%s:auditor", space))
	}
	for _, space := range flag.SpacesManager {
		groups = append(groups, fmt.Sprintf("%s:manager", space))
	}
	groups = append(groups, flag.SpaceGuids...)
	groups = append(groups, flag.SpaceGuidsLegacy...)
	return groups
}

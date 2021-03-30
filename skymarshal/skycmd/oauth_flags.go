package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/flag"
	"github.com/concourse/dex/connector/oauth"
	multierror "github.com/hashicorp/go-multierror"
)

type OAuthFlags struct {
	DisplayName        string     `yaml:"display_name,omitempty"`
	ClientID           string     `yaml:"client_id,omitempty"`
	ClientSecret       string     `yaml:"client_secret,omitempty"`
	AuthURL            string     `yaml:"auth_url,omitempty"`
	TokenURL           string     `yaml:"token_url,omitempty"`
	UserInfoURL        string     `yaml:"userinfo_url,omitempty"`
	Scopes             []string   `yaml:"scope,omitempty"`
	GroupsKey          string     `yaml:"groups_key,omitempty"`
	UserIDKey          string     `yaml:"user_id_key,omitempty"`
	UserNameKey        string     `yaml:"user_name_key,omitempty"`
	CACerts            flag.Files `yaml:"ca_cert,omitempty"`
	InsecureSkipVerify bool       `yaml:"skip_ssl_validation,omitempty"`
}

func (flag *OAuthFlags) ID() string {
	return OAuthConnectorID
}

func (flag *OAuthFlags) Name() string {
	if flag.DisplayName != "" {
		return flag.DisplayName
	}
	return "OAuth2"
}

func (flag *OAuthFlags) Validate() error {
	var errs *multierror.Error

	if flag.AuthURL == "" {
		errs = multierror.Append(errs, errors.New("Missing auth-url"))
	}

	if flag.TokenURL == "" {
		errs = multierror.Append(errs, errors.New("Missing token-url"))
	}

	if flag.UserInfoURL == "" {
		errs = multierror.Append(errs, errors.New("Missing userinfo-url"))
	}

	if flag.ClientID == "" {
		errs = multierror.Append(errs, errors.New("Missing client-id"))
	}

	if flag.ClientSecret == "" {
		errs = multierror.Append(errs, errors.New("Missing client-secret"))
	}

	return errs.ErrorOrNil()
}

func (flag *OAuthFlags) Serialize(redirectURI string) ([]byte, error) {
	if err := flag.Validate(); err != nil {
		return nil, err
	}

	caCerts := []string{}
	for _, file := range flag.CACerts {
		caCerts = append(caCerts, file.Path())
	}

	config := oauth.Config{
		ClientID:           flag.ClientID,
		ClientSecret:       flag.ClientSecret,
		AuthorizationURL:   flag.AuthURL,
		TokenURL:           flag.TokenURL,
		UserInfoURL:        flag.UserInfoURL,
		Scopes:             flag.Scopes,
		UserIDKey:          flag.UserIDKey,
		RootCAs:            caCerts,
		InsecureSkipVerify: flag.InsecureSkipVerify,
		RedirectURI:        redirectURI,
	}

	config.ClaimMapping.PreferredUsernameKey = flag.UserNameKey
	config.ClaimMapping.UserNameKey = flag.UserNameKey
	config.ClaimMapping.GroupsKey = flag.GroupsKey

	return json.Marshal(config)
}

type OAuthTeamFlags struct {
	Users  []string `yaml:"users,omitempty" env:"CONCOURSE_MAIN_TEAM_OAUTH_USERS,CONCOURSE_MAIN_TEAM_OAUTH_USER" json:"users" long:"user" description:"A whitelisted OAuth2 user" value-name:"USERNAME"`
	Groups []string `yaml:"groups,omitempty" env:"CONCOURSE_MAIN_TEAM_OAUTH_GROUPS,CONCOURSE_MAIN_TEAM_OAUTH_GROUP" json:"groups" long:"group" description:"A whitelisted OAuth2 group" value-name:"GROUP_NAME"`
}

func (flag *OAuthTeamFlags) ID() string {
	return OAuthConnectorID
}

func (flag *OAuthTeamFlags) GetUsers() []string {
	return flag.Users
}

func (flag *OAuthTeamFlags) GetGroups() []string {
	return flag.Groups
}

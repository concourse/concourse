package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/dex/connector/oauth"
	"github.com/concourse/flag"
	multierror "github.com/hashicorp/go-multierror"
)

func init() {
	RegisterConnector(&Connector{
		id:         "oauth",
		config:     &OAuthFlags{},
		teamConfig: &OAuthTeamFlags{},
	})
}

type OAuthFlags struct {
	DisplayName        string      `long:"display-name" description:"The auth provider name displayed to users on the login page"`
	ClientID           string      `long:"client-id" description:"(Required) Client id"`
	ClientSecret       string      `long:"client-secret" description:"(Required) Client secret"`
	AuthURL            string      `long:"auth-url" description:"(Required) Authorization URL"`
	TokenURL           string      `long:"token-url" description:"(Required) Token URL"`
	UserInfoURL        string      `long:"userinfo-url" description:"(Required) UserInfo URL"`
	Scopes             []string    `long:"scope" description:"Any additional scopes that need to be requested during authorization"`
	GroupsKey          string      `long:"groups-key" default:"groups" description:"The groups key indicates which claim to use to map external groups to Concourse teams."`
	UserIDKey          string      `long:"user-id-key" default:"user_id" description:"The user id key indicates which claim to use to map an external user id to a Concourse user id."`
	UserNameKey        string      `long:"user-name-key" default:"user_name" description:"The user name key indicates which claim to use to map an external user name to a Concourse user name."`
	CACerts            []flag.File `long:"ca-cert" description:"CA Certificate"`
	InsecureSkipVerify bool        `long:"skip-ssl-validation" description:"Skip SSL validation"`
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

	return json.Marshal(oauth.Config{
		ClientID:           flag.ClientID,
		ClientSecret:       flag.ClientSecret,
		AuthorizationURL:   flag.AuthURL,
		TokenURL:           flag.TokenURL,
		UserInfoURL:        flag.UserInfoURL,
		Scopes:             flag.Scopes,
		GroupsKey:          flag.GroupsKey,
		UserIDKey:          flag.UserIDKey,
		UserNameKey:        flag.UserNameKey,
		RootCAs:            caCerts,
		InsecureSkipVerify: flag.InsecureSkipVerify,
		RedirectURI:        redirectURI,
	})
}

type OAuthTeamFlags struct {
	Users  []string `json:"users" long:"user" description:"A whitelisted OAuth2 user" value-name:"USERNAME"`
	Groups []string `json:"groups" long:"group" description:"A whitelisted OAuth2 group" value-name:"GROUP_NAME"`
}

func (flag *OAuthTeamFlags) GetUsers() []string {
	return flag.Users
}

func (flag *OAuthTeamFlags) GetGroups() []string {
	return flag.Groups
}

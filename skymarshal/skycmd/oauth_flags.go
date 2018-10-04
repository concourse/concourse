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
	GroupsKey          string      `long:"groups-key" description:"The groups key indicates which claim to use to map external groups to Concourse teams."`
	CACerts            []flag.File `long:"ca-cert" description:"CA Certificate"`
	InsecureSkipVerify bool        `long:"skip-ssl-validation" description:"Skip SSL validation"`
}

func (self *OAuthFlags) Name() string {
	if self.DisplayName != "" {
		return self.DisplayName
	} else {
		return "OAuth2"
	}
}

func (self *OAuthFlags) Validate() error {
	var errs *multierror.Error

	if self.AuthURL == "" {
		errs = multierror.Append(errs, errors.New("Missing auth-url"))
	}

	if self.TokenURL == "" {
		errs = multierror.Append(errs, errors.New("Missing token-url"))
	}

	if self.UserInfoURL == "" {
		errs = multierror.Append(errs, errors.New("Missing userinfo-url"))
	}

	if self.ClientID == "" {
		errs = multierror.Append(errs, errors.New("Missing client-id"))
	}

	if self.ClientSecret == "" {
		errs = multierror.Append(errs, errors.New("Missing client-secret"))
	}

	return errs.ErrorOrNil()
}

func (self *OAuthFlags) Serialize(redirectURI string) ([]byte, error) {
	if err := self.Validate(); err != nil {
		return nil, err
	}

	caCerts := []string{}
	for _, file := range self.CACerts {
		caCerts = append(caCerts, file.Path())
	}

	return json.Marshal(oauth.Config{
		ClientID:           self.ClientID,
		ClientSecret:       self.ClientSecret,
		AuthorizationURL:   self.AuthURL,
		TokenURL:           self.TokenURL,
		UserInfoURL:        self.UserInfoURL,
		Scopes:             self.Scopes,
		GroupsKey:          self.GroupsKey,
		RootCAs:            caCerts,
		InsecureSkipVerify: self.InsecureSkipVerify,
		RedirectURI:        redirectURI,
	})
}

type OAuthTeamFlags struct {
	Users  []string `json:"users" long:"user" description:"List of whitelisted OAuth2 users" value-name:"USERNAME"`
	Groups []string `json:"groups" long:"group" description:"List of whitelisted OAuth2 groups" value-name:"GROUP_NAME"`
}

func (self *OAuthTeamFlags) IsValid() bool {
	return len(self.Users) > 0 || len(self.Groups) > 0
}

func (self *OAuthTeamFlags) GetUsers() []string {
	return self.Users
}

func (self *OAuthTeamFlags) GetGroups() []string {
	return self.Groups
}

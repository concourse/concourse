package auth

import "github.com/concourse/atc/db"

type AuthType string

type AuthProvider struct {
	BasicAuth    *BasicAuth
	GitHubAuth   *GitHubAuth
	UAAAuth      *UAAAuth
	GenericOAuth *GenericOAuth
}

const (
	AuthTypeBasic AuthType = "basic"
	AuthTypeOAuth AuthType = "oauth"
)

type authWrapper struct {
	auths []AuthProvider
	teamDB db.TeamDB
}

func NewAuthWrapper(
	authProviders []AuthProvider,
	teamDB db.TeamDB
) AuthWrapper (
	return &authWrapper{
		auths: AuthProviders,
		teamDB: teamDB,
	}
)

type AuthWrapper interface {
	Wrap()
}

func (a authWrapper) Wrap() {
	team, found, err := teamDB.GetTeam()
	if err != nil || !found {
		return false
	}
	for auth := range a.Auths {
		switch auth{
		case BasicAuth:
			NewBasicAuthValidator(team).IsAuthenticated(r)
		}
		}
}

type BasicAuth struct {
	BasicAuthUsername string `json:"basic_auth_username"`
	BasicAuthPassword string `json:"basic_auth_password"`
}

type GitHubAuth struct {
	ClientID      string          `json:"client_id"`
	ClientSecret  string          `json:"client_secret"`
	Organizations []string        `json:"organizations"`
	Teams         []db.GitHubTeam `json:"teams"`
	Users         []string        `json:"users"`
	AuthURL       string          `json:"auth_url"`
	TokenURL      string          `json:"token_url"`
	APIURL        string          `json:"api_url"`
}

type UAAAuth struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	AuthURL      string   `json:"auth_url"`
	TokenURL     string   `json:"token_url"`
	CFSpaces     []string `json:"cf_spaces"`
	CFURL        string   `json:"cf_url"`
	CFCACert     string   `json:"cf_ca_cert"`
}

type GenericOAuth struct {
	AuthURL       string            `json:"auth_url"`
	AuthURLParams map[string]string `json:"auth_url_params"`
	TokenURL      string            `json:"token_url"`
	ClientID      string            `json:"client_id"`
	ClientSecret  string            `json:"client_secret"`
	DisplayName   string            `json:"display_name"`
	Scope         string            `json:"scope"`
}

package auth

import (
	"github.com/concourse/atc/db"
)

type AuthType string
type AuthProvider string


const (
	AuthTypeBasic AuthType = "basicAuth"
	AuthTypeOAuth AuthType = "oauth"

	AuthProviderGithub AuthProvider = "githubAuthProvider"
	AuthProviderUAAAuth AuthProvider = "uaaAuthProvider"
	AuthProviderBasic AuthProvider = "basicAuthProvider"
	AuthTypeOAuth AuthProvider = "oauthProvider"
)

type authWrapper struct {
	auths []AuthProvider
	teamDB db.TeamDB
}

func NewAuthWrapper(
	authProviders []AuthProvider,
	teamDB db.TeamDB,
) AuthWrapper {
	return &authWrapper{
		auths:  authProviders,
		teamDB: teamDB,
	}
}


type AuthWrapper interface {
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

package db

import (
	"encoding/json"

	"golang.org/x/crypto/bcrypt"
)

type Team struct {
	Name  string
	Admin bool

	BasicAuth  *BasicAuth  `json:"basic_auth"`
	GitHubAuth *GitHubAuth `json:"github_auth"`
	UAAAuth    *UAAAuth    `json:"uaa_auth"`
}

type BasicAuth struct {
	BasicAuthUsername string `json:"basic_auth_username"`
	BasicAuthPassword string `json:"basic_auth_password"`
}

func (auth *BasicAuth) EncryptedJSON() (string, error) {
	var result *BasicAuth
	if auth != nil && auth.BasicAuthUsername != "" && auth.BasicAuthPassword != "" {
		encryptedPw, err := bcrypt.GenerateFromPassword([]byte(auth.BasicAuthPassword), 4)
		if err != nil {
			return "", err
		}
		result = &BasicAuth{
			BasicAuthPassword: string(encryptedPw),
			BasicAuthUsername: auth.BasicAuthUsername,
		}
	}

	json, err := json.Marshal(result)
	return string(json), err
}

type GitHubAuth struct {
	ClientID      string       `json:"client_id"`
	ClientSecret  string       `json:"client_secret"`
	Organizations []string     `json:"organizations"`
	Teams         []GitHubTeam `json:"teams"`
	Users         []string     `json:"users"`
	AuthURL       string       `json:"auth_url"`
	TokenURL      string       `json:"token_url"`
	APIURL        string       `json:"api_url"`
}

type GitHubTeam struct {
	OrganizationName string `json:"organization_name"`
	TeamName         string `json:"team_name"`
}

type SavedTeam struct {
	ID int
	Team
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

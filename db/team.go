package db

import (
	"encoding/json"

	"github.com/concourse/atc/auth"

	"golang.org/x/crypto/bcrypt"
)

type Team struct {
	Name  string
	Admin bool

	AuthWrapper auth.AuthWrapper

	// BasicAuth    *BasicAuth    `json:"basic_auth"`
	// GitHubAuth   *GitHubAuth   `json:"github_auth"`
	// UAAAuth      *UAAAuth      `json:"uaa_auth"`
	// GenericOAuth *GenericOAuth `json:"genericoauth_auth"`
}

func (t Team) IsAuthConfigured() bool {
	//loop through all methods and dont panic
	// return t.BasicAuth != nil || t.GitHubAuth != nil || t.UAAAuth != nil
	return t.AuthWrapper.CheckAuth()
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

type GitHubTeam struct {
	OrganizationName string `json:"organization_name"`
	TeamName         string `json:"team_name"`
}

type SavedTeam struct {
	ID int
	Team
}

package atc

import (
	"errors"
)

var (
	ErrAuthConfigEmpty   = errors.New("auth config for the team must not be empty")
	ErrAuthConfigInvalid = errors.New("auth config for the team does not have users and groups configured")
)

type Team struct {
	ID   int      `json:"id,omitempty"`
	Name string   `json:"name,omitempty"`
	Auth TeamAuth `json:"auth,omitempty"`
}

func (team Team) Validate() error {
	return team.Auth.Validate()
}

type TeamAuth map[string]map[string][]string

func (auth TeamAuth) Validate() error {
	if len(auth) == 0 {
		return ErrAuthConfigEmpty
	}

	for _, config := range auth {
		users := config["users"]
		groups := config["groups"]

		if len(users) == 0 && len(groups) == 0 {
			return ErrAuthConfigInvalid
		}
	}

	return nil
}

package atc

import (
	"errors"
)

var (
	ErrAuthConfigEmpty = errors.New("auth config must not be empty")
	ErrAuthConfigInvalid = errors.New("users and groups are not provided for a specified role")
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

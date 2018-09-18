package migrations

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
)

func (self *migrations) Up_1528470872() error {

	type team struct {
		id    int64
		name  string
		auth  []byte
		nonce sql.NullString
	}

	tx, err := self.DB.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("ALTER TABLE teams RENAME COLUMN auth TO legacy_auth")
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec("ALTER TABLE teams ADD COLUMN auth text")
	if err != nil {
		tx.Rollback()
		return err
	}

	rows, err := tx.Query("SELECT id, name, legacy_auth, nonce FROM teams")
	if err != nil {
		tx.Rollback()
		return err
	}

	teams := []team{}

	for rows.Next() {
		team := team{}

		if err = rows.Scan(&team.id, &team.name, &team.auth, &team.nonce); err != nil {
			tx.Rollback()
			return err
		}

		teams = append(teams, team)
	}

	mustBeUniqueAmongstAllTeams := map[string]map[string]map[string][]string{
		"basicauth": map[string]map[string][]string{
			"username": map[string][]string{},
		},
	}

	mustBeSameAmongstAllTeams := map[string]map[string]map[string][]string{
		"github": map[string]map[string][]string{
			"auth_url":  map[string][]string{},
			"token_url": map[string][]string{},
			"api_url":   map[string][]string{},
		},
		"uaa": map[string]map[string][]string{
			"auth_url":  map[string][]string{},
			"token_url": map[string][]string{},
			"cf_url":    map[string][]string{},
		},
		"gitlab": map[string]map[string][]string{
			"auth_url":  map[string][]string{},
			"token_url": map[string][]string{},
			"api_url":   map[string][]string{},
		},
		"oauth": map[string]map[string][]string{
			"auth_url":  map[string][]string{},
			"token_url": map[string][]string{},
		},
		"oauth_oidc": map[string]map[string][]string{
			"auth_url":  map[string][]string{},
			"token_url": map[string][]string{},
		},
	}

	for _, team := range teams {

		var noncense *string
		if team.nonce.Valid {
			noncense = &team.nonce.String
		}

		decryptedAuth, err := self.Strategy.Decrypt(string(team.auth), noncense)
		if err != nil {
			tx.Rollback()
			return err
		}

		var authConfig map[string]interface{}
		if err = json.Unmarshal(decryptedAuth, &authConfig); err != nil {
			tx.Rollback()
			return err
		}

		if authConfig == nil {
			authConfig = map[string]interface{}{}
		}

		newGroups := []string{}
		newUsers := []string{}

		for provider, rawConfig := range authConfig {

			for key, set := range mustBeSameAmongstAllTeams[provider] {
				if parsedConfig, ok := rawConfig.(map[string]interface{}); ok {
					if value, ok := parsedConfig[key].(string); ok {
						_, valuePresent := set[value]
						if valuePresent {
							set[value] = append(set[value], team.name)
						} else {
							set[value] = []string{team.name}
						}
					}
				}
			}

			for key, set := range mustBeUniqueAmongstAllTeams[provider] {
				if parsedConfig, ok := rawConfig.(map[string]interface{}); ok {
					if value, parseOk := parsedConfig[key].(string); parseOk {
						_, valuePresent := set[value]
						if valuePresent {
							set[value] = append(set[value], team.name)
						} else {
							set[value] = []string{team.name}
						}
					}
				}
			}

			switch provider {
			case "github":
				var config struct {
					Organizations []string `mapstructure:"organizations"`
					Teams         []struct {
						OrganizationName string `mapstructure:"organization_name"`
						TeamName         string `mapstructure:"team_name"`
					} `mapstructure:"teams"`
					Users []string `mapstructure:"users"`
				}
				if err = mapstructure.Decode(rawConfig, &config); err != nil {
					tx.Rollback()
					return err
				}

				for _, team := range config.Teams {
					newGroups = append(newGroups, provider+":"+team.OrganizationName+":"+team.TeamName)
				}
				for _, org := range config.Organizations {
					newGroups = append(newGroups, provider+":"+org)
				}
				for _, user := range config.Users {
					newUsers = append(newUsers, provider+":"+user)
				}

			case "basicauth":
				var config struct {
					Username string `mapstructure:"username"`
				}
				if err = mapstructure.Decode(rawConfig, &config); err != nil {
					tx.Rollback()
					return err
				}

				newUsers = append(newUsers, "local:"+config.Username)

			case "uaa":
				var config struct {
					Spaces []string `mapstructure:"cf_spaces"`
				}
				if err = mapstructure.Decode(rawConfig, &config); err != nil {
					tx.Rollback()
					return err
				}

				for _, space := range config.Spaces {
					newGroups = append(newGroups, "cf:"+space)
				}

			case "gitlab":
				var config struct {
					Groups []string `mapstructure:"groups"`
				}
				if err = mapstructure.Decode(rawConfig, &config); err != nil {
					tx.Rollback()
					return err
				}

				for _, group := range config.Groups {
					newGroups = append(newGroups, "gitlab:"+group)
				}

			case "oauth":
				var config struct {
					Scope string `mapstructure:"scope"`
				}
				if err = mapstructure.Decode(rawConfig, &config); err != nil {
					tx.Rollback()
					return err
				}

				newGroups = append(newGroups, "oauth:"+config.Scope)

			case "oauth_oidc":
				var config struct {
					UserID []string `mapstructure:"user_id"`
					Groups []string `mapstructure:"groups"`
				}
				if err = mapstructure.Decode(rawConfig, &config); err != nil {
					tx.Rollback()
					return err
				}

				for _, user := range config.UserID {
					newUsers = append(newUsers, "oidc:"+user)
				}
				for _, group := range config.Groups {
					newGroups = append(newGroups, "oidc:"+group)
				}

			case "bitbucket-server", "bitbucket-cloud":
				tx.Rollback()
				return errors.New("Bitbucket is no longer supported")
			}
		}

		newAuth, err := json.Marshal(map[string][]string{
			"users":  newUsers,
			"groups": newGroups,
		})
		if err != nil {
			tx.Rollback()
			return err
		}

		_, err = tx.Exec("UPDATE teams SET auth = $1 WHERE id = $2", newAuth, team.id)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	errorMessage := ""
	for provider, keys := range mustBeSameAmongstAllTeams {
		for key, values := range keys {
			if len(values) > 1 {
				errorMessage += fmt.Sprintf("Non-unique value of '%s' for auth provider '%s' breaks migration: ", key, provider)
				offendingTeams := []string{}
				for value, teams := range values {
					offendingTeams = append(offendingTeams, fmt.Sprintf("teams %v have value '%s'", teams, value))
				}
				errorMessage += strings.Join(offendingTeams, ", ")
				errorMessage += "\n"
			}
		}
	}
	for provider, keys := range mustBeUniqueAmongstAllTeams {
		for key, values := range keys {
			for value, teams := range values {
				if len(teams) > 1 {
					errorMessage += fmt.Sprintf("Multiple teams having the same value, '%s', of '%s' for auth provider '%s' breaks migration. Offending teams: %v\n", value, key, provider, teams)
				}
			}
		}
	}
	if errorMessage != "" {
		tx.Rollback()
		return fmt.Errorf("Problems in your database caused the migration to fail:\n\n%s", errorMessage)
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return err
	}

	return nil
}

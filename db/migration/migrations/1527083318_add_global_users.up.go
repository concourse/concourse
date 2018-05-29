package migrations

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/mitchellh/mapstructure"
)

func (self *migrations) Up_1527083318() error {

	type team struct {
		id    int64
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

	rows, err := tx.Query("SELECT id, legacy_auth, nonce FROM teams")
	if err != nil {
		tx.Rollback()
		return err
	}

	teams := []team{}

	for rows.Next() {
		team := team{}

		if err = rows.Scan(&team.id, &team.auth, &team.nonce); err != nil {
			tx.Rollback()
			return err
		}

		teams = append(teams, team)
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

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return err
	}

	return nil
}

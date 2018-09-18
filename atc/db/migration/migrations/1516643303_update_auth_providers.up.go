package migrations

import (
	"database/sql"
	"encoding/json"
)

func (self *migrations) Up_1516643303() error {

	type team struct {
		id        int64
		basicAuth []byte
		auth      []byte
		nonce     sql.NullString
	}

	tx, err := self.DB.Begin()
	if err != nil {
		return err
	}

	rows, err := tx.Query("SELECT id, basic_auth, auth, nonce FROM teams")
	if err != nil {
		return err
	}

	teams := []team{}

	for rows.Next() {
		team := team{}

		if err = rows.Scan(&team.id, &team.basicAuth, &team.auth, &team.nonce); err != nil {
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
		json.Unmarshal(decryptedAuth, &authConfig)

		if authConfig == nil {
			authConfig = map[string]interface{}{}
		}

		var basicAuthConfig map[string]string
		json.Unmarshal(team.basicAuth, &basicAuthConfig)

		if basicAuthConfig == nil {
			basicAuthConfig = map[string]string{}
		}

		username := basicAuthConfig["basic_auth_username"]
		password := basicAuthConfig["basic_auth_password"]

		if username != "" && password != "" {
			authConfig["basicauth"] = map[string]string{
				"username": username,
				"password": password,
			}
		}

		if len(authConfig) == 0 {
			authConfig["noauth"] = map[string]bool{
				"noauth": true,
			}
		}

		newAuth, err := json.Marshal(authConfig)
		if err != nil {
			tx.Rollback()
			return err
		}

		encryptedAuth, noncense, err := self.Strategy.Encrypt(newAuth)

		_, err = tx.Exec("UPDATE teams SET auth = $1, nonce = $2 WHERE id = $3", encryptedAuth, noncense, team.id)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	_, err = tx.Exec("ALTER TABLE teams DROP COLUMN IF EXISTS basic_auth")
	if err != nil {
		tx.Rollback()
		return err
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return err
	}

	return nil
}

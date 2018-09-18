package migrations

import (
	"database/sql"
	"encoding/json"
	"errors"
)

func (self *migrations) Down_1516643303() error {

	type team struct {
		id    int64
		auth  []byte
		nonce sql.NullString
	}

	tx, err := self.DB.Begin()
	if err != nil {
		return err
	}

	rows, err := tx.Query("SELECT id, auth, nonce FROM teams")
	if err != nil {
		return err
	}

	teams := []team{}

	for rows.Next() {
		team := team{}

		if err = rows.Scan(&team.id, &team.auth, &team.nonce); err != nil {
			return err
		}

		teams = append(teams, team)
	}

	_, err = tx.Exec("ALTER TABLE teams ADD COLUMN basic_auth json")
	if err != nil {
		tx.Rollback()
		return err
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
		err = json.Unmarshal(decryptedAuth, &authConfig)
		if err != nil {
			tx.Rollback()
			return err
		}

		var basicAuthConfig map[string]string

		if config, ok := authConfig["basicauth"]; ok {
			if configMap, ok := config.(map[string]interface{}); ok {
				basicAuthConfig = map[string]string{}
				basicAuthConfig["basic_auth_username"] = configMap["username"].(string)
				basicAuthConfig["basic_auth_password"] = configMap["password"].(string)
			} else {
				tx.Rollback()
				return errors.New("malformed basicauth provider")
			}
		}

		delete(authConfig, "noauth")
		delete(authConfig, "basicauth")

		newAuth, err := json.Marshal(authConfig)
		if err != nil {
			tx.Rollback()
			return err
		}

		newBasicAuth, err := json.Marshal(basicAuthConfig)
		if err != nil {
			tx.Rollback()
			return err
		}

		encryptedAuth, noncense, err := self.Strategy.Encrypt(newAuth)

		_, err = tx.Exec("UPDATE teams SET basic_auth = $1, auth = $2, nonce = $3 WHERE id = $4", newBasicAuth, encryptedAuth, noncense, team.id)
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

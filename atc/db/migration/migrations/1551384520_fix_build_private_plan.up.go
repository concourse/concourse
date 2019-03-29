package migrations

import (
	"database/sql"
	"encoding/json"
)

func (self *migrations) Up_1550160079() error {

	type build struct {
		id    int
		plan  sql.NullString
		nonce sql.NullString
	}

	tx, err := self.DB.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	rows, err := tx.Query("SELECT id, private_plan, nonce FROM builds WHERE private_plan IS NOT NULL")
	if err != nil {
		return err
	}

	builds := []build{}
	for rows.Next() {

		build := build{}
		if err = rows.Scan(&build.id, &build.plan, &build.nonce); err != nil {
			return err
		}

		if build.plan.Valid {
			builds = append(builds, build)
		}
	}

	for _, build := range builds {

		var noncense *string
		if build.nonce.Valid {
			noncense = &build.nonce.String
		}

		decrypted, err := self.Strategy.Decrypt(build.plan.String, noncense)
		if err != nil {
			return err
		}

		var payload map[string]interface{}
		err = json.Unmarshal(decrypted, &payload)
		if err != nil {
			return err
		}

		fixed, err := json.Marshal(payload["plan"])
		if err != nil {
			return err
		}

		encrypted, newnonce, err := self.Strategy.Encrypt(fixed)
		if err != nil {
			return err
		}

		_, err = tx.Exec("UPDATE builds SET private_plan = $1, nonce = $2 WHERE id = $3", encrypted, newnonce, build.id)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

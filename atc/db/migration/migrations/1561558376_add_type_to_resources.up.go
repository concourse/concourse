package migrations

import (
	"database/sql"
	"encoding/json"
)

func (self *migrations) Up_1561558376() error {
	type resource struct {
		id     int
		config string
		nonce  sql.NullString
	}

	tx, err := self.DB.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec("ALTER TABLE resources ADD COLUMN type text")
	if err != nil {
		return err
	}

	rows, err := tx.Query("SELECT id, config, nonce FROM resources")
	if err != nil {
		return err
	}

	resources := []resource{}
	for rows.Next() {

		resource := resource{}
		if err = rows.Scan(&resource.id, &resource.config, &resource.nonce); err != nil {
			return err
		}

		resources = append(resources, resource)
	}

	for _, resource := range resources {

		var noncense *string
		if resource.nonce.Valid {
			noncense = &resource.nonce.String
		}

		decrypted, err := self.Strategy.Decrypt(resource.config, noncense)
		if err != nil {
			return err
		}

		var payload struct {
			Type string `json:"type"`
		}

		err = json.Unmarshal(decrypted, &payload)
		if err != nil {
			return err
		}

		_, err = tx.Exec("UPDATE resources SET type = $1 WHERE id = $2", payload.Type, resource.id)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec("ALTER TABLE resources ALTER COLUMN type SET NOT NULL")
	if err != nil {
		return err
	}

	return tx.Commit()
}

package migrations

import (
	"database/sql"
	"encoding/json"
)

type V6ResourceConfigVersion struct {
	Name    string            `json:"name"`
	Version map[string]string `json:"version,omitempty"`
}

func (self *migrations) Up_1585079293() error {
	tx, err := self.DB.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec("ALTER TABLE resource_pins ADD COLUMN config boolean NOT NULL DEFAULT false")
	if err != nil {
		return err
	}

	rows, err := tx.Query("SELECT id, config, nonce FROM resources WHERE active = true")
	if err != nil {
		return err
	}

	configPinnedVersions := make(map[int]map[string]string)
	for rows.Next() {
		var configBlob []byte
		var nonce sql.NullString
		var resourceID int

		err = rows.Scan(&resourceID, &configBlob, &nonce)
		if err != nil {
			return err
		}

		var noncense *string
		if nonce.Valid {
			noncense = &nonce.String
		}

		decrypted, err := self.Strategy.Decrypt(string(configBlob), noncense)
		if err != nil {
			return err
		}

		var config V6ResourceConfigVersion
		err = json.Unmarshal(decrypted, &config)
		if err != nil {
			return err
		}

		if config.Version != nil {
			configPinnedVersions[resourceID] = config.Version
		}
	}

	for resourceID, version := range configPinnedVersions {
		versionJSON, err := json.Marshal(version)
		if err != nil {
			return err
		}

		_, err = tx.Exec(`
			INSERT INTO resource_pins (resource_id, version, config, comment_text)
			VALUES ($1, $2, true, '')
			ON CONFLICT (resource_id) DO UPDATE SET version = EXCLUDED.version, config = EXCLUDED.config, comment_text = EXCLUDED.comment_text`, resourceID, versionJSON)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}


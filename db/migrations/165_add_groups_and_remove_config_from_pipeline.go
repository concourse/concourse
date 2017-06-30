package migrations

import (
	"database/sql"
	"encoding/json"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db/migration"
)

func AddGroupsAndRemoveConfigFromPipeline(strategy EncryptionStrategy) migration.Migrator {
	return func(tx migration.LimitedTx) error {
		_, err := tx.Exec(`
			ALTER TABLE pipelines
			ADD COLUMN groups json;
		`)
		if err != nil {
			return err
		}

		rows, err := tx.Query(`
			SELECT id, config, nonce
			FROM pipelines
		`)
		if err != nil {
			return err
		}

		defer rows.Close()

		pipelineGroups := map[int][]byte{}
		for rows.Next() {
			var (
				pipelineID     int
				pipelineConfig []byte
				nonce          sql.NullString
			)

			err := rows.Scan(&pipelineID, &pipelineConfig, &nonce)
			if err != nil {
				return err
			}

			var noncense *string
			if nonce.Valid {
				noncense = &nonce.String
			}

			decryptedConfig, err := strategy.Decrypt(string(pipelineConfig), noncense)
			if err != nil {
				return err
			}

			var config atc.Config
			err = json.Unmarshal(decryptedConfig, &config)
			if err != nil {
				return err
			}

			groups, err := json.Marshal(config.Groups)
			if err != nil {
				return err
			}

			pipelineGroups[pipelineID] = groups
		}

		for id, groups := range pipelineGroups {
			_, err := tx.Exec(`
		UPDATE pipelines
		SET groups = $1
		WHERE id = $2`, groups, id)
			if err != nil {
				return err
			}
		}

		_, err = tx.Exec(`
		ALTER TABLE pipelines
		DROP COLUMN config
	`)
		if err != nil {
			return err
		}

		_, err = tx.Exec(`
		ALTER TABLE pipelines
		DROP COLUMN nonce
	`)
		if err != nil {
			return err
		}

		return nil
	}
}

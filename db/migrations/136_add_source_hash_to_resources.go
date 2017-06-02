package migrations

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db/migration"
)

func AddSourceHashToResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
  ALTER TABLE resources
  ADD COLUMN source_hash text
`)
	if err != nil {
		return err
	}

	rows, err := tx.Query(`SELECT id, config FROM resources`)
	if err != nil {
		return err
	}

	defer rows.Close()

	resourceSourceHashes := map[int]string{}

	for rows.Next() {
		var resourceID int
		var resourceConfigJSON string

		err = rows.Scan(&resourceID, &resourceConfigJSON)
		if err != nil {
			return fmt.Errorf("failed to scan resource ID and resource config: %s", err)
		}

		var resourceConfig atc.ResourceConfig
		err = json.Unmarshal([]byte(resourceConfigJSON), &resourceConfig)
		if err != nil {
			return fmt.Errorf("failed to unmarshal resource config: %s", err)
		}

		sourceJSON, err := json.Marshal(resourceConfig.Source)
		if err != nil {
			return fmt.Errorf("failed to marshal resource source: %s", err)
		}

		sourceHash := fmt.Sprintf("%x", sha256.Sum256(sourceJSON))
		resourceSourceHashes[resourceID] = sourceHash
	}

	for resourceID, sourceHash := range resourceSourceHashes {
		_, err = tx.Exec(`
		UPDATE resources
		SET source_hash = $1
		WHERE id = $2
	`, sourceHash, resourceID)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(`
	ALTER TABLE resources
	ALTER COLUMN source_hash SET NOT NULL
`)
	if err != nil {
		return err
	}

	return nil
}

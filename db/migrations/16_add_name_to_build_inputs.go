package migrations

import "github.com/concourse/atc/db/migration"

func AddNameToBuildInputs(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE build_inputs ADD COLUMN name text`)
	if err != nil {
		return err
	}

	names := map[int]string{}

	rows, err := tx.Query(`
    SELECT i.versioned_resource_id, v.resource_name
    FROM build_inputs i, versioned_resources v
    WHERE v.id = i.versioned_resource_id
  `)
	if err != nil {
		return err
	}

	defer rows.Close()

	for rows.Next() {
		var vrID int
		var name string
		err := rows.Scan(&vrID, &name)
		if err != nil {
			return err
		}

		names[vrID] = name
	}

	for vrID, name := range names {
		_, err := tx.Exec(`
      UPDATE build_inputs
      SET name = $2
      WHERE versioned_resource_id = $1
    `, vrID, name)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(`ALTER TABLE build_inputs ALTER COLUMN name SET NOT NULL`)
	if err != nil {
		return err
	}

	return nil
}

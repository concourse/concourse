package db

import (
	"encoding/json"

	"github.com/concourse/atc"
)

func (db *SQLDB) SaveImageResourceVersion(buildID int, planID atc.PlanID, identifier VolumeIdentifier) error {
	version, err := json.Marshal(identifier.ResourceVersion)
	if err != nil {
		return err
	}

	result, err := db.conn.Exec(`
		UPDATE image_resource_versions
		SET version = $1, resource_hash = $4
		WHERE build_id = $2 AND plan_id = $3
	`, version, buildID, string(planID), identifier.ResourceHash)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		_, err := db.conn.Exec(`
			INSERT INTO image_resource_versions(version, build_id, plan_id, resource_hash)
			VALUES ($1, $2, $3, $4)
		`, version, buildID, string(planID), identifier.ResourceHash)
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *SQLDB) GetImageVolumeIdentifiersByBuildID(buildID int) ([]VolumeIdentifier, error) {
	rows, err := db.conn.Query(`
  	SELECT version, resource_hash
  	FROM image_resource_versions
  	WHERE build_id = $1
  `, buildID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var volumeIdentifiers []VolumeIdentifier

	for rows.Next() {
		var identifier VolumeIdentifier
		var marshalledVersion []byte

		err := rows.Scan(&marshalledVersion, &identifier.ResourceHash)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(marshalledVersion, &identifier.ResourceVersion)
		if err != nil {
			return nil, err
		}

		volumeIdentifiers = append(volumeIdentifiers, identifier)
	}

	return volumeIdentifiers, nil
}

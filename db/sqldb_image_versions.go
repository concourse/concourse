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

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec(`
			INSERT INTO image_resource_versions(version, build_id, plan_id, resource_hash)
			VALUES ($1, $2, $3, $4)
		`, version, buildID, string(planID), identifier.ResourceHash)

	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
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

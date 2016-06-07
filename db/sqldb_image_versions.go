package db

import "encoding/json"

func (db *SQLDB) GetImageResourceCacheIdentifiersByBuildID(buildID int) ([]ResourceCacheIdentifier, error) {
	rows, err := db.conn.Query(`
  	SELECT version, resource_hash
  	FROM image_resource_versions
  	WHERE build_id = $1
  `, buildID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var identifiers []ResourceCacheIdentifier

	for rows.Next() {
		var identifier ResourceCacheIdentifier
		var marshalledVersion []byte

		err := rows.Scan(&marshalledVersion, &identifier.ResourceHash)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(marshalledVersion, &identifier.ResourceVersion)
		if err != nil {
			return nil, err
		}

		identifiers = append(identifiers, identifier)
	}

	return identifiers, nil
}

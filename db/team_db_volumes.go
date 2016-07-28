package db

import "errors"

func (db *teamDB) GetVolumes() ([]SavedVolume, error) {
	err := db.expireVolumes()
	if err != nil {
		return nil, err
	}

	team, found, err := db.GetTeam()
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, errors.New("team-not-found")
	}

	rows, err := db.conn.Query(`
		SELECT
			v.worker_name,
			v.ttl,
			EXTRACT(epoch FROM v.expires_at - NOW()),
			v.handle,
			v.resource_version,
			v.resource_hash,
			v.id,
			v.original_volume_handle,
			v.output_name,
			v.replicated_from,
			v.path,
			v.host_path_version,
			v.size_in_bytes,
			c.ttl,
			v.team_id
		FROM volumes v
		LEFT JOIN containers c
			ON v.container_id = c.id
		LEFT JOIN teams t
			ON v.team_id = t.id
		WHERE v.team_id = $1
		OR v.team_id is null
	`, team.ID)
	if err != nil {
		return nil, err
	}

	volumes, err := scanVolumes(rows)
	return volumes, err
}

func (db *teamDB) expireVolumes() error {
	_, err := db.conn.Exec(`
		DELETE FROM volumes
		WHERE expires_at IS NOT NULL
		AND expires_at < NOW()
	`)
	return err
}

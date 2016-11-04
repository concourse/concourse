package db

import (
	"database/sql"
	"time"
)

const volumeJoins = `
LEFT JOIN containers c
	ON v.container_id = c.id
LEFT JOIN teams t
	ON v.team_id = t.id
`

func (db *SQLDB) ReapVolume(handle string) error {
	_, err := db.conn.Exec(`
		DELETE FROM volumes
		WHERE handle = $1
	`, handle)
	return err
}

func (db *SQLDB) GetVolumes() ([]SavedVolume, error) {
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
		` + volumeJoins + `
		WHERE (v.expires_at IS NULL OR v.expires_at > NOW())
		`)
	if err != nil {
		return nil, err
	}

	volumes, err := scanVolumes(rows)
	return volumes, err
}

func (db *SQLDB) GetVolumesForOneOffBuildImageResources() ([]SavedVolume, error) {
	rows, err := db.conn.Query(`
		SELECT DISTINCT
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
		FROM volumes v ` + volumeJoins + `
			INNER JOIN image_resource_versions i
				ON i.version = v.resource_version
				AND i.resource_hash = v.resource_hash
			INNER JOIN builds b
				ON b.id = i.build_id
		WHERE b.job_id IS NULL
		AND (v.expires_at IS NULL OR v.expires_at > NOW())
	`)
	if err != nil {
		return nil, err
	}

	volumes, err := scanVolumes(rows)
	return volumes, err
}

func (db *SQLDB) ReapExpiredVolumes() error { //TODO probably lose this, maybe the whole reaping altogether
	_, err := db.conn.Exec(`
		DELETE FROM volumes
		WHERE expires_at IS NOT NULL
		AND expires_at < NOW()
	`)
	return err
}

func scanVolumes(rows *sql.Rows) ([]SavedVolume, error) {
	defer rows.Close()

	volumes := []SavedVolume{}

	for rows.Next() {
		var (
			volume               SavedVolume
			ttlSeconds           *float64
			versionJSON          sql.NullString
			resourceHash         sql.NullString
			originalVolumeHandle sql.NullString
			outputName           sql.NullString
			replicationName      sql.NullString
			path                 sql.NullString
			hostPathVersion      sql.NullString
			teamID               sql.NullInt64
		)

		err := rows.Scan(
			&volume.WorkerName,
			&volume.TTL,
			&ttlSeconds,
			&volume.Handle,
			&versionJSON,
			&resourceHash,
			&volume.ID,
			&originalVolumeHandle,
			&outputName,
			&replicationName,
			&path,
			&hostPathVersion,
			&volume.SizeInBytes,
			&volume.ContainerTTL,
			&teamID,
		)
		if err != nil {
			return []SavedVolume{}, err
		}

		if ttlSeconds != nil {
			volume.ExpiresIn = time.Duration(*ttlSeconds) * time.Second
		}

		if teamID.Valid {
			volume.TeamID = int(teamID.Int64)
		}

		volumes = append(volumes, volume)
	}

	return volumes, nil
}

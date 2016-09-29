package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const volumeJoins = `
LEFT JOIN containers c
	ON v.container_id = c.id
LEFT JOIN teams t
	ON v.team_id = t.id
`

func (db *SQLDB) InsertVolume(data Volume) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	var resourceVersion []byte

	columns := []string{"worker_name", "ttl", "state", "handle"}
	params := []interface{}{data.WorkerName, data.TTL, data.Handle}
	values := []string{"$1", "$2", "'created'", "$3"}

	if data.TTL == 0 {
		columns = append(columns, "expires_at")
		values = append(values, "NULL")
	} else {
		columns = append(columns, "expires_at")
		params = append(params, fmt.Sprintf("%d second", int(data.TTL.Seconds())))
		values = append(values, fmt.Sprintf("NOW() + $%d::INTERVAL", len(params)))
	}

	if data.TeamID != 0 {
		columns = append(columns, "team_id")
		params = append(params, data.TeamID)
		values = append(values, fmt.Sprintf("$%d", len(params)))
	}

	switch {
	case data.Identifier.ResourceCache != nil:
		resourceVersion, err = json.Marshal(data.Identifier.ResourceCache.ResourceVersion)
		if err != nil {
			return err
		}

		columns = append(columns, "resource_version")
		params = append(params, resourceVersion)
		values = append(values, fmt.Sprintf("$%d", len(params)))

		columns = append(columns, "resource_hash")
		params = append(params, data.Identifier.ResourceCache.ResourceHash)
		values = append(values, fmt.Sprintf("$%d", len(params)))
	case data.Identifier.COW != nil:
		columns = append(columns, "original_volume_handle")
		params = append(params, data.Identifier.COW.ParentVolumeHandle)
		values = append(values, fmt.Sprintf("$%d", len(params)))
	case data.Identifier.Output != nil:
		columns = append(columns, "output_name")
		params = append(params, data.Identifier.Output.Name)
		values = append(values, fmt.Sprintf("$%d", len(params)))
	case data.Identifier.Import != nil:
		columns = append(columns, "path")
		params = append(params, data.Identifier.Import.Path)
		values = append(values, fmt.Sprintf("$%d", len(params)))

		columns = append(columns, "host_path_version")
		params = append(params, data.Identifier.Import.Version)
		values = append(values, fmt.Sprintf("$%d", len(params)))

	case data.Identifier.Replication != nil:
		columns = append(columns, "replicated_from")
		params = append(params, data.Identifier.Replication.ReplicatedVolumeHandle)
		values = append(values, fmt.Sprintf("$%d", len(params)))
	}

	_, err = tx.Exec(
		fmt.Sprintf(
			`
				INSERT INTO volumes(
					%s
				) VALUES (
					%s
				)
			`,
			strings.Join(columns, ", "),
			strings.Join(values, ", "),
		), params...)
	if err != nil {
		if strings.Contains(err.Error(), `duplicate key value violates unique constraint "volumes_worker_name_handle_key"`) {
			return nil
		}

		return err
	}

	return tx.Commit()
}

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

func (db *SQLDB) GetVolumesByIdentifier(id VolumeIdentifier) ([]SavedVolume, error) {
	conditions := []string{"(v.expires_at IS NULL OR v.expires_at > NOW())"}
	params := []interface{}{}

	addParam := func(column string, param interface{}) {
		conditions = append(conditions, fmt.Sprintf("v.%s = $%d", column, len(params)+1))
		params = append(params, param)
	}

	switch {
	case id.ResourceCache != nil:
		resourceVersion, err := json.Marshal(id.ResourceCache.ResourceVersion)
		if err != nil {
			return nil, err
		}
		addParam("resource_version", resourceVersion)
		addParam("resource_hash", id.ResourceCache.ResourceHash)
	case id.COW != nil:
		addParam("original_volume_handle", id.COW.ParentVolumeHandle)
	case id.Output != nil:
		addParam("output_name", id.Output.Name)
	case id.Import != nil:
		addParam("path", id.Import.Path)
		addParam("worker_name", id.Import.WorkerName)
		if id.Import.Version != nil {
			addParam("host_path_version", id.Import.Version)
		}
	case id.Replication != nil:
		addParam("replicated_from", id.Replication.ReplicatedVolumeHandle)
	}

	statement := `
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
		FROM volumes v` + volumeJoins

	statement += "WHERE " + strings.Join(conditions, " AND ")
	statement += "ORDER BY id ASC"
	rows, err := db.conn.Query(statement, params...)
	if err != nil {
		return nil, err
	}

	savedVolumes, err := scanVolumes(rows)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return savedVolumes, nil
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

func (db *SQLDB) SetVolumeTTLAndSizeInBytes(handle string, ttl time.Duration, sizeInBytes int64) error {
	if ttl == 0 {
		_, err := db.conn.Exec(`
			UPDATE volumes
			SET expires_at = null, ttl = 0, size_in_bytes = $2
			WHERE handle = $1
		`, handle, sizeInBytes)

		return err
	}

	interval := fmt.Sprintf("%d second", int(ttl.Seconds()))

	_, err := db.conn.Exec(`
		UPDATE volumes
		SET expires_at = NOW() + $1::INTERVAL,
		ttl = $2,
		size_in_bytes = $3
		WHERE handle = $4
	`, interval, ttl, sizeInBytes, handle)

	return err
}

func (db *SQLDB) SetVolumeTTL(handle string, ttl time.Duration) error {
	if ttl == 0 {
		_, err := db.conn.Exec(`
			UPDATE volumes
			SET expires_at = null, ttl = 0
			WHERE handle = $1
		`, handle)

		return err
	}

	interval := fmt.Sprintf("%d second", int(ttl.Seconds()))

	_, err := db.conn.Exec(`
		UPDATE volumes
		SET expires_at = NOW() + $1::INTERVAL,
		ttl = $2
		WHERE handle = $3
	`, interval, ttl, handle)

	return err
}

func (db *SQLDB) GetVolumeTTL(handle string) (time.Duration, bool, error) {
	var ttl time.Duration

	err := db.conn.QueryRow(`
		SELECT ttl
		FROM volumes
		WHERE handle = $1
	`, handle).Scan(&ttl)
	if err == sql.ErrNoRows {
		return 0, false, nil
	} else if err != nil {
		return 0, false, err
	}

	return ttl, true, nil
}

func (db *SQLDB) ReapExpiredVolumes() error {
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

		switch {
		case versionJSON.Valid && resourceHash.Valid:
			var cacheID ResourceCacheIdentifier

			err = json.Unmarshal([]byte(versionJSON.String), &cacheID.ResourceVersion)
			if err != nil {
				return []SavedVolume{}, err
			}

			cacheID.ResourceHash = resourceHash.String

			volume.Volume.Identifier.ResourceCache = &cacheID
		case originalVolumeHandle.Valid:
			volume.Volume.Identifier.COW = &COWIdentifier{
				ParentVolumeHandle: originalVolumeHandle.String,
			}
		case outputName.Valid:
			volume.Volume.Identifier.Output = &OutputIdentifier{
				Name: outputName.String,
			}
		case replicationName.Valid:
			volume.Volume.Identifier.Replication = &ReplicationIdentifier{
				ReplicatedVolumeHandle: replicationName.String,
			}
		case path.Valid:
			volume.Volume.Identifier.Import = &ImportIdentifier{
				Path:       path.String,
				WorkerName: volume.WorkerName,
				Version:    &hostPathVersion.String,
			}
		}

		volumes = append(volumes, volume)
	}

	return volumes, nil
}

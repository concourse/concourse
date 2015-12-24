package db

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (db *SQLDB) InsertVolume(data Volume) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	var resourceVersion []byte

	resourceVersion, err = json.Marshal(data.ResourceVersion)
	if err != nil {
		return err
	}

	interval := fmt.Sprintf("%d second", int(data.TTL.Seconds()))

	defer tx.Rollback()
	if data.TTL == 0 {
		_, err = tx.Exec(`
		INSERT INTO volumes(
			worker_id,
			expires_at,
			ttl,
			handle,
			resource_version,
			resource_hash
		) VALUES (
			$1,
		  NULL,
			$2,
			$3,
			$4,
			$5
		)
	`, data.WorkerID,
			data.TTL,
			data.Handle,
			resourceVersion,
			data.ResourceHash,
		)
	} else {
		_, err = tx.Exec(`
		INSERT INTO volumes(
			worker_id,
			expires_at,
			ttl,
			handle,
			resource_version,
			resource_hash
		) VALUES (
			$1,
			NOW() + $2::INTERVAL,
			$3,
			$4,
			$5,
			$6
		)
	`, data.WorkerID,
			interval,
			data.TTL,
			data.Handle,
			resourceVersion,
			data.ResourceHash,
		)
	}
	if err != nil {
		if strings.Contains(err.Error(), `duplicate key value violates unique constraint "volumes_worker_id_handle_key"`) {
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
	// reap expired volumes
	_, err := db.conn.Exec(`
		DELETE FROM volumes
		WHERE expires_at IS NOT NULL
		AND expires_at < NOW()
	`)
	if err != nil {
		return nil, err
	}

	rows, err := db.conn.Query(`
		SELECT
			v.worker_id,
			w.name,
			v.ttl,
			EXTRACT(epoch FROM v.expires_at - NOW()),
			v.handle,
			v.resource_version,
			v.resource_hash,
			v.id
		FROM volumes v
		JOIN workers w
			ON w.id = v.worker_id
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	volumes := []SavedVolume{}

	for rows.Next() {
		var volume SavedVolume
		var ttlSeconds *float64
		var versionJSON []byte

		err := rows.Scan(&volume.WorkerID, &volume.WorkerName, &volume.TTL, &ttlSeconds, &volume.Handle, &versionJSON, &volume.ResourceHash, &volume.ID)
		if err != nil {
			return nil, err
		}

		if ttlSeconds != nil {
			volume.ExpiresIn = time.Duration(*ttlSeconds) * time.Second
		}

		err = json.Unmarshal(versionJSON, &volume.ResourceVersion)
		if err != nil {
			return nil, err
		}

		volumes = append(volumes, volume)
	}

	return volumes, nil
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

func (db *SQLDB) GetVolumeTTL(handle string) (time.Duration, error) {
	var ttl time.Duration

	err := db.conn.QueryRow(`
		SELECT ttl
		FROM volumes
		WHERE handle = $1
	`, handle).Scan(&ttl)

	return ttl, err
}

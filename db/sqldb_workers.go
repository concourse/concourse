package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/concourse/atc"
)

func (db *SQLDB) Workers() ([]SavedWorker, error) {
	// reap expired workers
	_, err := db.conn.Exec(`
		DELETE FROM workers
		WHERE expires IS NOT NULL
		AND expires < NOW()
	`)
	if err != nil {
		return nil, err
	}

	// TODO: Clean this up after people have upgraded and we can guarantee the name field is present and populated
	// select remaining workers
	rows, err := db.conn.Query(`
		SELECT id, EXTRACT(epoch FROM expires - NOW()), addr, baggageclaim_url, active_containers, resource_types, platform, tags,
			CASE
				WHEN COALESCE(name, '') = '' then addr
				ELSE name
			END as name
		FROM workers
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	savedWorkers := []SavedWorker{}
	for rows.Next() {
		savedWorker, err := scanWorker(rows)
		if err != nil {
			return nil, err
		}

		savedWorkers = append(savedWorkers, savedWorker)
	}

	return savedWorkers, nil
}

func (db *SQLDB) GetWorker(id int) (SavedWorker, bool, error) {
	// reap expired workers
	_, err := db.conn.Exec(`
		DELETE FROM workers
		WHERE expires IS NOT NULL
		AND expires < NOW()
	`)
	if err != nil {
		return SavedWorker{}, false, err
	}

	// TODO: Clean this up after people have upgraded and we can guarantee the name field is present and populated
	savedWorker, err := scanWorker(db.conn.QueryRow(`
		SELECT id, EXTRACT(epoch FROM expires - NOW()), addr, baggageclaim_url, active_containers, resource_types, platform, tags, name
		FROM workers
		WHERE id = $1
	`, id))

	if err != nil {
		if err == sql.ErrNoRows {
			return SavedWorker{}, false, nil
		}
		return SavedWorker{}, false, err
	}

	return savedWorker, true, nil
}

func (db *SQLDB) saveBuildEvent(tx *sql.Tx, buildID int, event atc.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	_, err = tx.Exec(fmt.Sprintf(`
		INSERT INTO build_events (event_id, build_id, type, version, payload)
		VALUES (nextval('%s'), $1, $2, $3, $4)
	`, buildEventSeq(buildID)), buildID, string(event.EventType()), string(event.Version()), payload)
	if err != nil {
		return err
	}

	return nil
}

func (db *SQLDB) SaveWorker(info WorkerInfo, ttl time.Duration) (SavedWorker, error) {
	var savedWorker SavedWorker
	resourceTypes, err := json.Marshal(info.ResourceTypes)
	if err != nil {
		return SavedWorker{}, err
	}

	tags, err := json.Marshal(info.Tags)
	if err != nil {
		return SavedWorker{}, err
	}

	if ttl == 0 {
		row := db.conn.QueryRow(`
			UPDATE workers
			SET addr = $1, expires = NULL, active_containers = $2, resource_types = $3, platform = $4, tags = $5, baggageclaim_url = $6, name = $7
			WHERE name = $7 OR addr = $1
			RETURNING  id, EXTRACT(epoch FROM expires - NOW()), addr, baggageclaim_url, active_containers, resource_types, platform, tags, name
		`, info.GardenAddr, info.ActiveContainers, resourceTypes, info.Platform, tags, info.BaggageclaimURL, info.Name)

		savedWorker, err = scanWorker(row)
		if err == sql.ErrNoRows {
			row = db.conn.QueryRow(`
				INSERT INTO workers (addr, expires, active_containers, resource_types, platform, tags, baggageclaim_url, name)
				VALUES ($1, NULL, $2, $3, $4, $5, $6, $7)
				RETURNING  id, EXTRACT(epoch FROM expires - NOW()), addr, baggageclaim_url, active_containers, resource_types, platform, tags, name
			`, info.GardenAddr, info.ActiveContainers, resourceTypes, info.Platform, tags, info.BaggageclaimURL, info.Name)
			savedWorker, err = scanWorker(row)
		}
		if err != nil {
			return SavedWorker{}, err
		}
	} else {
		interval := fmt.Sprintf("%d second", int(ttl.Seconds()))

		row := db.conn.QueryRow(`
			UPDATE workers
			SET addr = $1, expires = NOW() + $2::INTERVAL, active_containers = $3, resource_types = $4, platform = $5, tags = $6, baggageclaim_url = $7, name = $8
			WHERE name = $8 OR addr = $1
			RETURNING  id, EXTRACT(epoch FROM expires - NOW()), addr, baggageclaim_url, active_containers, resource_types, platform, tags, name
		`, info.GardenAddr, interval, info.ActiveContainers, resourceTypes, info.Platform, tags, info.BaggageclaimURL, info.Name)

		savedWorker, err = scanWorker(row)
		if err == sql.ErrNoRows {
			row := db.conn.QueryRow(`
				INSERT INTO workers (addr, expires, active_containers, resource_types, platform, tags, baggageclaim_url, name)
				VALUES ($1, NOW() + $2::INTERVAL, $3, $4, $5, $6, $7, $8)
				RETURNING  id, EXTRACT(epoch FROM expires - NOW()), addr, baggageclaim_url, active_containers, resource_types, platform, tags, name
			`, info.GardenAddr, interval, info.ActiveContainers, resourceTypes, info.Platform, tags, info.BaggageclaimURL, info.Name)
			savedWorker, err = scanWorker(row)
		}
		if err != nil {
			return SavedWorker{}, err
		}
	}

	return savedWorker, nil
}

func scanWorker(row scannable) (SavedWorker, error) {
	info := SavedWorker{}

	var ttlSeconds *float64
	var resourceTypes []byte
	var tags []byte

	err := row.Scan(&info.ID, &ttlSeconds, &info.GardenAddr, &info.BaggageclaimURL, &info.ActiveContainers, &resourceTypes, &info.Platform, &tags, &info.Name)
	if err != nil {
		return SavedWorker{}, err
	}

	if ttlSeconds != nil {
		info.ExpiresIn = time.Duration(*ttlSeconds) * time.Second
	}

	err = json.Unmarshal(resourceTypes, &info.ResourceTypes)
	if err != nil {
		return SavedWorker{}, err
	}

	err = json.Unmarshal(tags, &info.Tags)
	if err != nil {
		return SavedWorker{}, err
	}

	return info, nil
}

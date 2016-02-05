package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

var workerColumns = "EXTRACT(epoch FROM expires - NOW()), addr, baggageclaim_url, active_containers, resource_types, platform, tags, name"

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

	rows, err := db.conn.Query(`
		SELECT ` + workerColumns + `
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

func (db *SQLDB) GetWorker(name string) (SavedWorker, bool, error) {
	// reap expired workers
	_, err := db.conn.Exec(`
		DELETE FROM workers
		WHERE expires IS NOT NULL
		AND expires < NOW()
	`)
	if err != nil {
		return SavedWorker{}, false, err
	}

	savedWorker, err := scanWorker(db.conn.QueryRow(`
		SELECT `+workerColumns+`
		FROM workers
		WHERE name = $1
	`, name))

	if err != nil {
		if err == sql.ErrNoRows {
			return SavedWorker{}, false, nil
		}
		return SavedWorker{}, false, err
	}

	return savedWorker, true, nil
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
			RETURNING  `+workerColumns,
			info.GardenAddr, info.ActiveContainers, resourceTypes, info.Platform, tags, info.BaggageclaimURL, info.Name)

		savedWorker, err = scanWorker(row)
		if err == sql.ErrNoRows {
			row = db.conn.QueryRow(`
				INSERT INTO workers (addr, expires, active_containers, resource_types, platform, tags, baggageclaim_url, name)
				VALUES ($1, NULL, $2, $3, $4, $5, $6, $7)
				RETURNING `+workerColumns,
				info.GardenAddr, info.ActiveContainers, resourceTypes, info.Platform, tags, info.BaggageclaimURL, info.Name)
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
			RETURNING `+workerColumns,
			info.GardenAddr, interval, info.ActiveContainers, resourceTypes, info.Platform, tags, info.BaggageclaimURL, info.Name)

		savedWorker, err = scanWorker(row)
		if err == sql.ErrNoRows {
			row := db.conn.QueryRow(`
				INSERT INTO workers (addr, expires, active_containers, resource_types, platform, tags, baggageclaim_url, name)
				VALUES ($1, NOW() + $2::INTERVAL, $3, $4, $5, $6, $7, $8)
				RETURNING `+workerColumns,
				info.GardenAddr, interval, info.ActiveContainers, resourceTypes, info.Platform, tags, info.BaggageclaimURL, info.Name)
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

	err := row.Scan(&ttlSeconds, &info.GardenAddr, &info.BaggageclaimURL, &info.ActiveContainers, &resourceTypes, &info.Platform, &tags, &info.Name)
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

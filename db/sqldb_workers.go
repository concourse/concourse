package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var workerColumns = "EXTRACT(epoch FROM expires - NOW()), addr, baggageclaim_url, http_proxy_url, https_proxy_url, no_proxy, active_containers, resource_types, platform, tags, name, start_time"

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

func (db *SQLDB) FindWorkerCheckResourceTypeVersion(workerName string, checkType string) (string, bool, error) {
	savedWorker, found, err := db.GetWorker(workerName)

	if err != nil {
		return "", false, err
	}

	if !found {
		return "", false, errors.New("worker-not-found")
	}

	for _, workerResourceType := range savedWorker.ResourceTypes {
		if checkType == workerResourceType.Type {
			return workerResourceType.Version, true, nil
		}
	}

	return "", false, nil
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

	expires := "NULL"
	if ttl != 0 {
		expires = fmt.Sprintf(`NOW() + '%d second'::INTERVAL`, int(ttl.Seconds()))
	}

	row := db.conn.QueryRow(`
			UPDATE workers
			SET addr = $1, expires = `+expires+`, active_containers = $2, resource_types = $3, platform = $4, tags = $5, baggageclaim_url = $6, http_proxy_url = $7, https_proxy_url = $8, no_proxy = $9, name = $10, start_time = $11
			WHERE name = $10 OR addr = $1
			RETURNING  `+workerColumns,
		info.GardenAddr, info.ActiveContainers, resourceTypes, info.Platform, tags, info.BaggageclaimURL, info.HTTPProxyURL, info.HTTPSProxyURL, info.NoProxy, info.Name, info.StartTime)

	savedWorker, err = scanWorker(row)
	if err == sql.ErrNoRows {
		row = db.conn.QueryRow(`
				INSERT INTO workers (addr, expires, active_containers, resource_types, platform, tags, baggageclaim_url, http_proxy_url, https_proxy_url, no_proxy, name, start_time)
				VALUES ($1, `+expires+`, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
				RETURNING `+workerColumns,
			info.GardenAddr, info.ActiveContainers, resourceTypes, info.Platform, tags, info.BaggageclaimURL, info.HTTPProxyURL, info.HTTPSProxyURL, info.NoProxy, info.Name, info.StartTime)
		savedWorker, err = scanWorker(row)
	}
	if err != nil {
		return SavedWorker{}, err
	}

	return savedWorker, nil
}

func scanWorker(row scannable) (SavedWorker, error) {
	info := SavedWorker{}

	var ttlSeconds *float64
	var resourceTypes []byte
	var tags []byte

	var httpProxyURL sql.NullString
	var httpsProxyURL sql.NullString
	var noProxy sql.NullString

	err := row.Scan(&ttlSeconds, &info.GardenAddr, &info.BaggageclaimURL, &httpProxyURL, &httpsProxyURL, &noProxy, &info.ActiveContainers, &resourceTypes, &info.Platform, &tags, &info.Name, &info.StartTime)
	if err != nil {
		return SavedWorker{}, err
	}

	if ttlSeconds != nil {
		info.ExpiresIn = time.Duration(*ttlSeconds) * time.Second
	}

	if httpProxyURL.Valid {
		info.HTTPProxyURL = httpProxyURL.String
	}

	if httpsProxyURL.Valid {
		info.HTTPSProxyURL = httpsProxyURL.String
	}

	if noProxy.Valid {
		info.NoProxy = noProxy.String
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

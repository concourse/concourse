package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var workerColumns = "EXTRACT(epoch FROM expires - NOW()), addr, baggageclaim_url, http_proxy_url, https_proxy_url, no_proxy, active_containers, resource_types, platform, tags, w.name as name, start_time, t.name as team_name"
var actualWorkerColumns = "EXTRACT(epoch FROM expires - NOW()), addr, baggageclaim_url, http_proxy_url, https_proxy_url, no_proxy, active_containers, resource_types, platform, tags, name, start_time"

func (db *SQLDB) Workers() ([]SavedWorker, error) {
	err := reapExpiredWorkers(db.conn)
	if err != nil {
		return nil, err
	}

	rows, err := db.conn.Query(`
		SELECT ` + workerColumns + `
		FROM workers as w
		LEFT OUTER JOIN teams as t ON t.id = w.team_id
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	savedWorkers := []SavedWorker{}
	for rows.Next() {
		savedWorker, err := scanWorker(rows, true)
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
	err := reapExpiredWorkers(db.conn)
	if err != nil {
		return SavedWorker{}, false, err
	}

	savedWorker, err := scanWorker(db.conn.QueryRow(`
		SELECT `+workerColumns+`
		FROM workers as w
		LEFT OUTER JOIN teams as t ON t.id = team_id
		WHERE w.name = $1
	`, name), true)

	if err != nil {
		if err == sql.ErrNoRows {
			return SavedWorker{}, false, nil
		}
		return SavedWorker{}, false, err
	}

	return savedWorker, true, nil
}

func (db *SQLDB) SaveWorker(info WorkerInfo, ttl time.Duration) (SavedWorker, error) {
	var teamID *int
	if info.Team != "" {
		err := db.conn.QueryRow(`
			SELECT id
			FROM teams
			WHERE name = $1
		`, info.Team).Scan(&teamID)
		if err != nil {
			return SavedWorker{}, err
		}
	}

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
			SET addr = $1, expires = `+expires+`, active_containers = $2, resource_types = $3, platform = $4, tags = $5, baggageclaim_url = $6, http_proxy_url = $7, https_proxy_url = $8, no_proxy = $9, name = $10, start_time = $11, team_id = $12
			WHERE name = $10 OR addr = $1
			RETURNING  `+actualWorkerColumns,
		info.GardenAddr, info.ActiveContainers, resourceTypes, info.Platform, tags, info.BaggageclaimURL, info.HTTPProxyURL, info.HTTPSProxyURL, info.NoProxy, info.Name, info.StartTime, teamID)

	savedWorker, err = scanWorker(row, false)
	if err == sql.ErrNoRows {
		row = db.conn.QueryRow(`
			INSERT INTO workers (addr, expires, active_containers, resource_types, platform, tags, baggageclaim_url, http_proxy_url, https_proxy_url, no_proxy, name, start_time, team_id)
			VALUES ($1, `+expires+`, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING `+actualWorkerColumns,
			info.GardenAddr, info.ActiveContainers, resourceTypes, info.Platform, tags, info.BaggageclaimURL, info.HTTPProxyURL, info.HTTPSProxyURL, info.NoProxy, info.Name, info.StartTime, teamID)
		savedWorker, err = scanWorker(row, false)
	}
	if err != nil {
		return SavedWorker{}, err
	}

	savedWorker.Team = info.Team
	return savedWorker, nil
}

func reapExpiredWorkers(dbConn Conn) error {
	_, err := dbConn.Exec(`
		DELETE FROM workers
		WHERE expires IS NOT NULL
		AND expires < NOW()
	`)
	return err
}

func scanWorker(row scannable, scanTeam bool) (SavedWorker, error) {
	info := SavedWorker{}

	var ttlSeconds *float64
	var resourceTypes []byte
	var tags []byte

	var httpProxyURL sql.NullString
	var httpsProxyURL sql.NullString
	var noProxy sql.NullString
	var teamName sql.NullString

	var err error
	if scanTeam {
		err = row.Scan(&ttlSeconds, &info.GardenAddr, &info.BaggageclaimURL, &httpProxyURL, &httpsProxyURL, &noProxy, &info.ActiveContainers, &resourceTypes, &info.Platform, &tags, &info.Name, &info.StartTime, &teamName)
	} else {
		err = row.Scan(&ttlSeconds, &info.GardenAddr, &info.BaggageclaimURL, &httpProxyURL, &httpsProxyURL, &noProxy, &info.ActiveContainers, &resourceTypes, &info.Platform, &tags, &info.Name, &info.StartTime)
	}
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

	if teamName.Valid {
		info.Team = teamName.String
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

package db

import (
	"database/sql"
	"encoding/json"
	"time"
)

var workerColumns = "EXTRACT(epoch FROM expires - NOW()), addr, baggageclaim_url, http_proxy_url, https_proxy_url, no_proxy, active_containers, resource_types, platform, tags, w.name as name, start_time, t.name as team_name, team_id"
var actualWorkerColumns = "EXTRACT(epoch FROM expires - NOW()), addr, baggageclaim_url, http_proxy_url, https_proxy_url, no_proxy, active_containers, resource_types, platform, tags, name, start_time"

func (db *SQLDB) Workers() ([]SavedWorker, error) {
	rows, err := db.conn.Query(`
		SELECT ` + workerColumns + `
		FROM workers as w
		LEFT OUTER JOIN teams as t ON t.id = w.team_id
		WHERE (expires IS NULL OR expires > NOW())
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

func (db *SQLDB) GetWorker(name string) (SavedWorker, bool, error) {
	savedWorker, err := scanWorker(db.conn.QueryRow(`
		SELECT `+workerColumns+`
		FROM workers as w
		LEFT OUTER JOIN teams as t ON t.id = team_id
		WHERE w.name = $1
		AND (expires IS NULL OR expires > NOW())
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
	panic("REPLACED BY DBNG")
}

func (db *SQLDB) ReapExpiredWorkers() error {
	panic("REPLACED BY DBNG")
}

func scanWorker(row scannable, scanTeam bool) (SavedWorker, error) {
	info := SavedWorker{}

	var expiresAt *time.Time
	var resourceTypes []byte
	var tags []byte

	var httpProxyURL sql.NullString
	var httpsProxyURL sql.NullString
	var noProxy sql.NullString
	var teamName sql.NullString
	var teamID sql.NullInt64
	var err error

	if scanTeam {
		err = row.Scan(&expiresAt, &info.GardenAddr, &info.BaggageclaimURL, &httpProxyURL, &httpsProxyURL, &noProxy, &info.ActiveContainers, &resourceTypes, &info.Platform, &tags, &info.Name, &info.StartTime, &teamName, &teamID)
	} else {
		err = row.Scan(&expiresAt, &info.GardenAddr, &info.BaggageclaimURL, &httpProxyURL, &httpsProxyURL, &noProxy, &info.ActiveContainers, &resourceTypes, &info.Platform, &tags, &info.Name, &info.StartTime)
	}
	if err != nil {
		return SavedWorker{}, err
	}

	if expiresAt != nil {
		info.ExpiresAt = time.Time(*expiresAt)
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
		info.TeamName = teamName.String
	}

	if teamID.Valid {
		info.TeamID = int(teamID.Int64)
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

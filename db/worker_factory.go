package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
)

//go:generate counterfeiter . WorkerFactory

type WorkerFactory interface {
	GetWorker(name string) (Worker, bool, error)
	SaveWorker(atcWorker atc.Worker, ttl time.Duration) (Worker, error)
	HeartbeatWorker(worker atc.Worker, ttl time.Duration) (Worker, error)
	Workers() ([]Worker, error)
	VisibleWorkers([]string) ([]Worker, error)
}

type workerFactory struct {
	conn Conn
}

func NewWorkerFactory(conn Conn) WorkerFactory {
	return &workerFactory{
		conn: conn,
	}
}

var workersQuery = psql.Select(`
		w.name,
		w.version,
		w.addr,
		w.state,
		w.baggageclaim_url,
		w.certs_path,
		w.http_proxy_url,
		w.https_proxy_url,
		w.no_proxy,
		w.active_containers,
		w.resource_types,
		w.platform,
		w.tags,
		t.name,
		w.team_id,
		w.start_time,
		w.expires
	`).
	From("workers w").
	LeftJoin("teams t ON w.team_id = t.id")

func (f *workerFactory) GetWorker(name string) (Worker, bool, error) {
	return getWorker(f.conn, workersQuery.Where(sq.Eq{"w.name": name}))
}

func (f *workerFactory) VisibleWorkers(teamNames []string) ([]Worker, error) {
	workersQuery := workersQuery.
		Where(sq.Or{
			sq.Eq{"t.name": teamNames},
			sq.Eq{"w.team_id": nil},
		})

	workers, err := getWorkers(f.conn, workersQuery)
	if err != nil {
		return nil, err
	}

	return workers, nil
}

func (f *workerFactory) Workers() ([]Worker, error) {
	return getWorkers(f.conn, workersQuery)
}

func getWorker(conn Conn, query sq.SelectBuilder) (Worker, bool, error) {
	row := query.
		RunWith(conn).
		QueryRow()

	worker := &worker{conn: conn}
	err := scanWorker(worker, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	return worker, true, nil
}

func getWorkers(conn Conn, query sq.SelectBuilder) ([]Worker, error) {
	rows, err := query.RunWith(conn).Query()
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	workers := []Worker{}

	for rows.Next() {
		worker := &worker{conn: conn}
		err := scanWorker(worker, rows)
		if err != nil {
			return nil, err
		}

		workers = append(workers, worker)
	}

	return workers, nil
}

func scanWorker(worker *worker, row scannable) error {
	var (
		version  sql.NullString
		addStr   sql.NullString
		state    string
		bcURLStr sql.NullString
		//	reaperAddr    sql.NullString
		certsPathStr  sql.NullString
		httpProxyURL  sql.NullString
		httpsProxyURL sql.NullString
		noProxy       sql.NullString
		resourceTypes []byte
		platform      sql.NullString
		tags          []byte
		teamName      sql.NullString
		teamID        sql.NullInt64
		startTime     sql.NullInt64
		expiresAt     *time.Time
	)

	err := row.Scan(
		&worker.name,
		&version,
		&addStr,
		&state,
		&bcURLStr,
		//	&reaperAddr,
		&certsPathStr,
		&httpProxyURL,
		&httpsProxyURL,
		&noProxy,
		&worker.activeContainers,
		&resourceTypes,
		&platform,
		&tags,
		&teamName,
		&teamID,
		&startTime,
		&expiresAt,
	)
	if err != nil {
		return err
	}

	if version.Valid {
		worker.version = &version.String
	}

	if addStr.Valid {
		worker.gardenAddr = &addStr.String
	}

	if bcURLStr.Valid {
		worker.baggageclaimURL = &bcURLStr.String
	}

	// if reaperAddr.Valid {
	// 	worker.reaperAddr = &reaperAddr.String
	// }

	if certsPathStr.Valid {
		worker.certsPath = &certsPathStr.String
	}

	worker.state = WorkerState(state)

	if startTime.Valid {
		worker.startTime = startTime.Int64
	}

	if expiresAt != nil {
		worker.expiresAt = *expiresAt
	}

	if httpProxyURL.Valid {
		worker.httpProxyURL = httpProxyURL.String
	}

	if httpsProxyURL.Valid {
		worker.httpsProxyURL = httpsProxyURL.String
	}

	if noProxy.Valid {
		worker.noProxy = noProxy.String
	}

	if teamName.Valid {
		worker.teamName = teamName.String
	}

	if teamID.Valid {
		worker.teamID = int(teamID.Int64)
	}

	if platform.Valid {
		worker.platform = platform.String
	}

	err = json.Unmarshal(resourceTypes, &worker.resourceTypes)
	if err != nil {
		return err
	}

	return json.Unmarshal(tags, &worker.tags)
}

func (f *workerFactory) HeartbeatWorker(atcWorker atc.Worker, ttl time.Duration) (Worker, error) {
	// In order to be able to calculate the ttl that we return to the caller
	// we must compare time.Now() to the worker.expires column
	// However, workers.expires column is a "timestamp (without timezone)"
	// So we format time.Now() without any timezone information and then
	// parse that using the same layout to strip the timezone information

	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}
	defer Rollback(tx)

	expires := "NULL"
	if ttl != 0 {
		expires = fmt.Sprintf(`NOW() + '%d second'::INTERVAL`, int(ttl.Seconds()))
	}

	cSQL, _, err := sq.Case("state").
		When("'landing'::worker_state", "'landing'::worker_state").
		When("'landed'::worker_state", "'landed'::worker_state").
		When("'retiring'::worker_state", "'retiring'::worker_state").
		Else("'running'::worker_state").
		ToSql()

	if err != nil {
		return nil, err
	}

	addrSQL, _, err := sq.Case("state").
		When("'landed'::worker_state", "NULL").
		Else("'" + atcWorker.GardenAddr + "'").
		ToSql()
	if err != nil {
		return nil, err
	}

	bcSQL, _, err := sq.Case("state").
		When("'landed'::worker_state", "NULL").
		Else("'" + atcWorker.BaggageclaimURL + "'").
		ToSql()
	if err != nil {
		return nil, err
	}

	// reapSQL, _, err := sq.Case("state").
	// 	When("'landed'::worker_state", "NULL").
	// 	Else("'" + atcWorker.ReaperAddr + "'").
	// 	ToSql()
	// if err != nil {
	// 	return nil, err
	// }

	_, err = psql.Update("workers").
		Set("expires", sq.Expr(expires)).
		Set("addr", sq.Expr("("+addrSQL+")")).
		Set("baggageclaim_url", sq.Expr("("+bcSQL+")")).
		//	Set("reaper_addr", sq.Expr("("+reapSQL+")")).
		Set("active_containers", atcWorker.ActiveContainers).
		Set("state", sq.Expr("("+cSQL+")")).
		Where(sq.Eq{"name": atcWorker.Name}).
		RunWith(tx).
		Exec()
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrWorkerNotPresent
		}
		return nil, err
	}

	row := workersQuery.Where(sq.Eq{"w.name": atcWorker.Name}).
		RunWith(tx).
		QueryRow()

	worker := &worker{conn: f.conn}
	err = scanWorker(worker, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrWorkerNotPresent
		}
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}
	return worker, nil

}

func (f *workerFactory) SaveWorker(atcWorker atc.Worker, ttl time.Duration) (Worker, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	savedWorker, err := saveWorker(tx, atcWorker, nil, ttl, f.conn)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return savedWorker, nil
}

func saveWorker(tx Tx, atcWorker atc.Worker, teamID *int, ttl time.Duration, conn Conn) (Worker, error) {
	resourceTypes, err := json.Marshal(atcWorker.ResourceTypes)
	if err != nil {
		return nil, err
	}

	tags, err := json.Marshal(atcWorker.Tags)
	if err != nil {
		return nil, err
	}

	expires := "NULL"
	if ttl != 0 {
		expires = fmt.Sprintf(`NOW() + '%d second'::INTERVAL`, int(ttl.Seconds()))
	}

	var oldTeamID sql.NullInt64

	var workerState WorkerState
	if atcWorker.State != "" {
		workerState = WorkerState(atcWorker.State)
	} else {
		workerState = WorkerStateRunning
	}

	var workerVersion *string
	if atcWorker.Version != "" {
		workerVersion = &atcWorker.Version
	}

	err = psql.Select("team_id").From("workers").Where(sq.Eq{
		"name": atcWorker.Name,
	}).RunWith(tx).QueryRow().Scan(&oldTeamID)

	if err != nil {
		if err == sql.ErrNoRows {
			_, err = psql.Insert("workers").
				Columns(
					"addr",
					"expires",
					"active_containers",
					"resource_types",
					"tags",
					"platform",
					"baggageclaim_url",
					// "reaper_addr",
					"certs_path",
					"http_proxy_url",
					"https_proxy_url",
					"no_proxy",
					"name",
					"version",
					"start_time",
					"team_id",
					"state",
				).
				Values(
					atcWorker.GardenAddr,
					sq.Expr(expires),
					atcWorker.ActiveContainers,
					resourceTypes,
					tags,
					atcWorker.Platform,
					atcWorker.BaggageclaimURL,
					//			atcWorker.ReaperAddr,
					atcWorker.CertsPath,
					atcWorker.HTTPProxyURL,
					atcWorker.HTTPSProxyURL,
					atcWorker.NoProxy,
					atcWorker.Name,
					workerVersion,
					atcWorker.StartTime,
					teamID,
					string(workerState),
				).
				RunWith(tx).
				Exec()
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else {
		if (oldTeamID.Valid == (teamID == nil)) ||
			(oldTeamID.Valid && (*teamID != int(oldTeamID.Int64))) {
			return nil, errors.New("update-of-other-teams-worker-not-allowed")
		}

		_, err = psql.Update("workers").
			Set("addr", atcWorker.GardenAddr).
			Set("expires", sq.Expr(expires)).
			Set("active_containers", atcWorker.ActiveContainers).
			Set("resource_types", resourceTypes).
			Set("tags", tags).
			Set("platform", atcWorker.Platform).
			Set("baggageclaim_url", atcWorker.BaggageclaimURL).
			//		Set("reaper_addr", atcWorker.ReaperAddr).
			Set("certs_path", atcWorker.CertsPath).
			Set("http_proxy_url", atcWorker.HTTPProxyURL).
			Set("https_proxy_url", atcWorker.HTTPSProxyURL).
			Set("no_proxy", atcWorker.NoProxy).
			Set("name", atcWorker.Name).
			Set("version", workerVersion).
			Set("start_time", atcWorker.StartTime).
			Set("state", string(workerState)).
			Where(sq.Eq{
				"name": atcWorker.Name,
			}).
			RunWith(tx).
			Exec()
		if err != nil {
			return nil, err
		}
	}

	var workerTeamID int
	if teamID != nil {
		workerTeamID = *teamID
	}

	savedWorker := &worker{
		name:            atcWorker.Name,
		version:         workerVersion,
		state:           workerState,
		gardenAddr:      &atcWorker.GardenAddr,
		baggageclaimURL: &atcWorker.BaggageclaimURL,
		//reaperAddr:       &atcWorker.ReaperAddr,
		certsPath:        atcWorker.CertsPath,
		httpProxyURL:     atcWorker.HTTPProxyURL,
		httpsProxyURL:    atcWorker.HTTPSProxyURL,
		noProxy:          atcWorker.NoProxy,
		activeContainers: atcWorker.ActiveContainers,
		resourceTypes:    atcWorker.ResourceTypes,
		platform:         atcWorker.Platform,
		tags:             atcWorker.Tags,
		teamName:         atcWorker.Team,
		teamID:           workerTeamID,
		startTime:        atcWorker.StartTime,
		conn:             conn,
	}

	workerBaseResourceTypeIDs := []int{}

	var (
		brt  BaseResourceType
		ubrt *UsedBaseResourceType
		uwrt *UsedWorkerResourceType
	)

	for _, resourceType := range atcWorker.ResourceTypes {
		workerResourceType := WorkerResourceType{
			Worker:  savedWorker,
			Image:   resourceType.Image,
			Version: resourceType.Version,
			BaseResourceType: &BaseResourceType{
				Name: resourceType.Type,
			},
		}

		brt = BaseResourceType{
			Name: resourceType.Type,
		}

		ubrt, err = brt.FindOrCreate(tx)
		if err != nil {
			return nil, err
		}

		_, err = psql.Delete("worker_base_resource_types").
			Where(sq.Eq{
				"worker_name":           atcWorker.Name,
				"base_resource_type_id": ubrt.ID,
			}).
			Where(sq.NotEq{
				"version": resourceType.Version,
			}).
			RunWith(tx).
			Exec()
		if err != nil {
			return nil, err
		}
		uwrt, err = workerResourceType.FindOrCreate(tx)
		if err != nil {
			return nil, err
		}

		workerBaseResourceTypeIDs = append(workerBaseResourceTypeIDs, uwrt.ID)
	}

	_, err = psql.Delete("worker_base_resource_types").
		Where(sq.Eq{
			"worker_name": atcWorker.Name,
		}).
		Where(sq.NotEq{
			"id": workerBaseResourceTypeIDs,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return nil, err
	}

	if atcWorker.CertsPath != nil {
		_, err := WorkerResourceCerts{
			WorkerName: atcWorker.Name,
			CertsPath:  *atcWorker.CertsPath,
		}.FindOrCreate(tx)

		if err != nil {
			return nil, err
		}
	}

	return savedWorker, nil
}

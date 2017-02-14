package dbng

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
	GetWorker(name string) (*Worker, bool, error)
	Workers() ([]*Worker, error)
	WorkersForTeam(teamName string) ([]*Worker, error)
	StallWorker(name string) (*Worker, error)
	StallUnresponsiveWorkers() ([]*Worker, error)
	DeleteFinishedRetiringWorkers() error
	LandFinishedLandingWorkers() error
	SaveWorker(worker atc.Worker, ttl time.Duration) (*Worker, error)
	LandWorker(name string) (*Worker, error)
	RetireWorker(name string) (*Worker, error)
	PruneWorker(name string) error
	DeleteWorker(name string) error
	HeartbeatWorker(worker atc.Worker, ttl time.Duration) (*Worker, error)
}

type workerFactory struct {
	conn Conn
}

func NewWorkerFactory(conn Conn) WorkerFactory {
	return &workerFactory{
		conn: conn,
	}
}

func (f *workerFactory) GetWorker(name string) (*Worker, bool, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer tx.Rollback()

	row := psql.Select(`
		w.name,
		w.addr,
		w.state,
		w.baggageclaim_url,
		w.http_proxy_url,
		w.https_proxy_url,
		w.no_proxy,
		w.active_containers,
		w.resource_types,
		w.platform,
		w.tags,
		w.team_id,
		w.start_time,
		t.name,
		EXTRACT(epoch FROM w.expires - NOW())
	`).
		From("workers w").
		LeftJoin("teams t ON w.team_id = t.id").
		Where(sq.Eq{"w.name": name}).
		RunWith(tx).
		QueryRow()

	worker, err := scanWorker(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	return worker, true, nil
}

var workersQuery = psql.Select(`
		w.name,
		w.addr,
		w.state,
		w.baggageclaim_url,
		w.http_proxy_url,
		w.https_proxy_url,
		w.no_proxy,
		w.active_containers,
		w.resource_types,
		w.platform,
		w.tags,
		w.team_id,
		w.start_time,
		t.name,
		EXTRACT(epoch FROM w.expires - NOW())
	`).
	From("workers w").
	LeftJoin("teams t ON w.team_id = t.id")

func (f *workerFactory) Workers() ([]*Worker, error) {
	return f.getWorkers(nil)
}

func (f *workerFactory) WorkersForTeam(teamName string) ([]*Worker, error) {
	return f.getWorkers(&teamName)
}

func (f *workerFactory) getWorkers(teamName *string) ([]*Worker, error) {
	selectWorkers := workersQuery

	if teamName != nil {
		selectWorkers = selectWorkers.Where(sq.Or{
			sq.Eq{"t.name": teamName},
			sq.Eq{"w.team_id": nil},
		})
	}

	query, args, err := selectWorkers.ToSql()

	if err != nil {
		return []*Worker{}, err
	}

	rows, err := f.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	workers := []*Worker{}

	for rows.Next() {
		worker, err := scanWorker(rows)
		if err != nil {
			return []*Worker{}, err
		}
		workers = append(workers, worker)
	}

	return workers, nil
}

func scanWorker(row scannable) (*Worker, error) {
	var (
		name          string
		addStr        sql.NullString
		state         string
		bcURLStr      sql.NullString
		httpProxyURL  sql.NullString
		httpsProxyURL sql.NullString
		noProxy       sql.NullString

		activeContainers int
		resourceTypes    []byte
		platform         sql.NullString
		tags             []byte
		teamID           sql.NullInt64
		startTime        int64

		teamName  sql.NullString
		expiresIn *float64
	)

	err := row.Scan(
		&name,
		&addStr,
		&state,
		&bcURLStr,
		&httpProxyURL,
		&httpsProxyURL,
		&noProxy,
		&activeContainers,
		&resourceTypes,
		&platform,
		&tags,
		&teamID,
		&startTime,
		&teamName,
		&expiresIn,
	)
	if err != nil {
		return nil, err
	}

	worker := Worker{
		Name:             name,
		State:            WorkerState(state),
		ActiveContainers: activeContainers,
		StartTime:        startTime,
	}

	if addStr.Valid {
		worker.GardenAddr = &addStr.String
	}

	if bcURLStr.Valid {
		worker.BaggageclaimURL = &bcURLStr.String
	}

	if expiresIn != nil {
		worker.ExpiresIn = time.Duration(*expiresIn) * time.Second
	}

	if httpProxyURL.Valid {
		worker.HTTPProxyURL = httpProxyURL.String
	}

	if httpsProxyURL.Valid {
		worker.HTTPSProxyURL = httpsProxyURL.String
	}

	if noProxy.Valid {
		worker.NoProxy = noProxy.String
	}

	if teamName.Valid {
		worker.TeamName = teamName.String
	}

	if teamID.Valid {
		worker.TeamID = int(teamID.Int64)
	}

	if platform.Valid {
		worker.Platform = platform.String
	}

	err = json.Unmarshal(resourceTypes, &worker.ResourceTypes)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(tags, &worker.Tags)
	if err != nil {
		return nil, err
	}
	return &worker, nil
}

func (f *workerFactory) LandWorker(name string) (*Worker, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var (
		workerName  string
		workerState string
	)

	cSql, _, err := sq.Case("state").
		When("'landed'::worker_state", "'landed'::worker_state").
		Else("'landing'::worker_state").
		ToSql()
	if err != nil {
		return nil, err
	}

	err = psql.Update("workers").
		Set("state", sq.Expr("("+cSql+")")).
		Where(sq.Eq{"name": name}).
		Suffix("RETURNING name, state").
		RunWith(tx).
		QueryRow().
		Scan(&workerName, &workerState)
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

	return &Worker{
		Name:  workerName,
		State: WorkerState(workerState),
	}, nil
}

func (f *workerFactory) RetireWorker(name string) (*Worker, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var (
		workerName  string
		workerState string
	)

	err = psql.Update("workers").
		SetMap(map[string]interface{}{
			"state": string(WorkerStateRetiring),
		}).
		Where(sq.Eq{"name": name}).
		Suffix("RETURNING name, state").
		RunWith(tx).
		QueryRow().
		Scan(&workerName, &workerState)
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

	return &Worker{
		Name:  workerName,
		State: WorkerState(workerState),
	}, nil
}

func (f *workerFactory) PruneWorker(name string) error {
	tx, err := f.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := sq.Delete("workers").
		Where(sq.Eq{
			"name": name,
		}).
		Where(sq.NotEq{
			"state": string(WorkerStateRunning),
		}).
		PlaceholderFormat(sq.Dollar).
		RunWith(tx).
		Exec()

	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		_, found, err := f.GetWorker(name)
		if err != nil {
			return err
		}

		if !found {
			return ErrWorkerNotPresent
		}

		return ErrCannotPruneRunningWorker
	}

	return nil
}

func (f *workerFactory) DeleteWorker(name string) error {
	tx, err := f.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sq.Delete("workers").
		Where(sq.Eq{
			"name": name,
		}).
		PlaceholderFormat(sq.Dollar).
		RunWith(tx).
		Exec()

	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (f *workerFactory) HeartbeatWorker(worker atc.Worker, ttl time.Duration) (*Worker, error) {
	// In order to be able to calculate the ttl that we return to the caller
	// we must compare time.Now() to the worker.expires column
	// However, workers.expires column is a "timestamp (without timezone)"
	// So we format time.Now() without any timezone information and then
	// parse that using the same layout to strip the timezone information
	layout := "Jan 2, 2006 15:04:05"
	nowStr := time.Now().Format(layout)
	now, err := time.Parse(layout, nowStr)
	if err != nil {
		return nil, err
	}

	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	expires := "NULL"
	if ttl != 0 {
		expires = fmt.Sprintf(`NOW() + '%d second'::INTERVAL`, int(ttl.Seconds()))
	}

	cSql, _, err := sq.Case("state").
		When("'landing'::worker_state", "'landing'::worker_state").
		When("'landed'::worker_state", "'landed'::worker_state").
		When("'retiring'::worker_state", "'retiring'::worker_state").
		Else("'running'::worker_state").
		ToSql()

	if err != nil {
		return nil, err
	}

	addrSql, _, err := sq.Case("state").
		When("'landed'::worker_state", "NULL").
		Else("'" + worker.GardenAddr + "'").
		ToSql()
	if err != nil {
		return nil, err
	}

	bcSql, _, err := sq.Case("state").
		When("'landed'::worker_state", "NULL").
		Else("'" + worker.BaggageclaimURL + "'").
		ToSql()
	if err != nil {
		return nil, err
	}

	var (
		workerName       string
		workerStateStr   string
		activeContainers int
		expiresAt        time.Time
		addrStr          sql.NullString
		bcURLStr         sql.NullString
	)

	err = psql.Update("workers").
		Set("expires", sq.Expr(expires)).
		Set("addr", sq.Expr("("+addrSql+")")).
		Set("baggageclaim_url", sq.Expr("("+bcSql+")")).
		Set("active_containers", worker.ActiveContainers).
		Set("state", sq.Expr("("+cSql+")")).
		Where(sq.Eq{"name": worker.Name}).
		Suffix("RETURNING name, addr, baggageclaim_url, state, expires, active_containers").
		RunWith(tx).
		QueryRow().
		Scan(&workerName, &addrStr, &bcURLStr, &workerStateStr, &expiresAt, &activeContainers)
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

	var addr *string
	if addrStr.Valid {
		addr = &addrStr.String
	}

	var bcURL *string
	if bcURLStr.Valid {
		bcURL = &bcURLStr.String
	}

	return &Worker{
		Name:             workerName,
		GardenAddr:       addr,
		BaggageclaimURL:  bcURL,
		State:            WorkerState(workerStateStr),
		ExpiresIn:        expiresAt.Sub(now),
		ActiveContainers: activeContainers,
	}, nil
}

func (f *workerFactory) StallWorker(name string) (*Worker, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var (
		workerName  string
		addrStr     sql.NullString
		bcURLStr    sql.NullString
		workerState string
	)
	err = psql.Update("workers").
		SetMap(map[string]interface{}{
			"state":            string(WorkerStateStalled),
			"expires":          nil,
			"addr":             nil,
			"baggageclaim_url": nil,
		}).
		Where(sq.Eq{"name": name}).
		Suffix("RETURNING name, addr, baggageclaim_url, state").
		RunWith(tx).
		QueryRow().
		Scan(&workerName, &addrStr, &bcURLStr, &workerState)
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

	var addr *string
	if addrStr.Valid {
		addr = &addrStr.String
	}

	var bcURL *string
	if bcURLStr.Valid {
		bcURL = &bcURLStr.String
	}

	return &Worker{
		Name:            workerName,
		GardenAddr:      addr,
		BaggageclaimURL: bcURL,
		State:           WorkerState(workerState),
	}, nil
}

func (f *workerFactory) StallUnresponsiveWorkers() ([]*Worker, error) {
	query, args, err := psql.Update("workers").
		SetMap(map[string]interface{}{
			"state":            string(WorkerStateStalled),
			"addr":             nil,
			"baggageclaim_url": nil,
			"expires":          nil,
		}).
		Where(sq.Eq{"state": string(WorkerStateRunning)}).
		Where(sq.Expr("expires < NOW()")).
		Suffix("RETURNING name, addr, baggageclaim_url, state").
		ToSql()
	if err != nil {
		return []*Worker{}, err
	}

	rows, err := f.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	workers := []*Worker{}

	for rows.Next() {
		var (
			name     string
			addrStr  sql.NullString
			bcURLStr sql.NullString
			state    string
		)

		err = rows.Scan(&name, &addrStr, &bcURLStr, &state)
		if err != nil {
			return nil, err
		}

		var addr *string
		if addrStr.Valid {
			addr = &addrStr.String
		}

		var bcURL *string
		if bcURLStr.Valid {
			bcURL = &bcURLStr.String
		}

		workers = append(workers, &Worker{
			Name:            name,
			GardenAddr:      addr,
			BaggageclaimURL: bcURL,
			State:           WorkerState(state),
		})
	}

	return workers, nil
}

func (f *workerFactory) DeleteFinishedRetiringWorkers() error {
	tx, err := f.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Squirrel does not have default support for subqueries in where clauses.
	// We hacked together a way to do it
	//
	// First we generate the subquery's SQL and args using
	// sq.Select instead of psql.Select so that we get
	// unordered placeholders instead of psql's ordered placeholders
	subQ, subQArgs, err := sq.Select("w.name").
		Distinct().
		From("builds b").
		Join("containers c ON b.id = c.build_id").
		Join("workers w ON w.name = c.worker_name").
		LeftJoin("jobs j ON j.id = b.job_id").
		Where(sq.Or{
			sq.Eq{
				"b.status": string(BuildStatusStarted),
			},
			sq.Eq{
				"b.status": string(BuildStatusPending),
			},
		}).
		Where(sq.Or{
			sq.Eq{
				"j.interruptible": false,
			},
			sq.Eq{
				"b.job_id": nil,
			},
		}).ToSql()

	if err != nil {
		return err
	}

	// Then we inject the subquery sql directly into
	// the where clause, and "add" the args from the
	// first query to the second query's args
	//
	// We use sq.Delete instead of psql.Delete for the same reason
	// but then change the placeholders using .PlaceholderFormat(sq.Dollar)
	// to go back to postgres's format
	_, err = sq.Delete("workers").
		Where(sq.Eq{
			"state": string(WorkerStateRetiring),
		}).
		Where("name NOT IN ("+subQ+")", subQArgs...).
		PlaceholderFormat(sq.Dollar).
		RunWith(tx).
		Exec()

	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (f *workerFactory) LandFinishedLandingWorkers() error {
	tx, err := f.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	subQ, subQArgs, err := sq.Select("w.name").
		Distinct().
		From("builds b").
		Join("containers c ON b.id = c.build_id").
		Join("workers w ON w.name = c.worker_name").
		LeftJoin("jobs j ON j.id = b.job_id").
		Where(sq.Or{
			sq.Eq{
				"b.status": string(BuildStatusStarted),
			},
			sq.Eq{
				"b.status": string(BuildStatusPending),
			},
		}).
		Where(sq.Or{
			sq.Eq{
				"j.interruptible": false,
			},
			sq.Eq{
				"b.job_id": nil,
			},
		}).ToSql()

	if err != nil {
		return err
	}

	_, err = sq.Update("workers").
		Set("state", string(WorkerStateLanded)).
		Set("addr", nil).
		Set("baggageclaim_url", nil).
		Where(sq.Eq{
			"state": string(WorkerStateLanding),
		}).
		Where("name NOT IN ("+subQ+")", subQArgs...).
		PlaceholderFormat(sq.Dollar).
		RunWith(tx).
		Exec()

	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (f *workerFactory) SaveWorker(worker atc.Worker, ttl time.Duration) (*Worker, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	savedWorker, err := saveWorker(tx, worker, nil, ttl)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return savedWorker, nil
}

func saveWorker(tx Tx, worker atc.Worker, teamID *int, ttl time.Duration) (*Worker, error) {
	resourceTypes, err := json.Marshal(worker.ResourceTypes)
	if err != nil {
		return nil, err
	}

	tags, err := json.Marshal(worker.Tags)
	if err != nil {
		return nil, err
	}

	expires := "NULL"
	if ttl != 0 {
		expires = fmt.Sprintf(`NOW() + '%d second'::INTERVAL`, int(ttl.Seconds()))
	}

	var oldTeamID sql.NullInt64

	err = psql.Select("team_id").From("workers").Where(sq.Eq{
		"name": worker.Name,
	}).RunWith(tx).QueryRow().Scan(&oldTeamID)

	var workerState WorkerState
	if worker.State != "" {
		workerState = WorkerState(worker.State)
	} else {
		workerState = WorkerStateRunning
	}

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
					"http_proxy_url",
					"https_proxy_url",
					"no_proxy",
					"name",
					"start_time",
					"team_id",
					"state",
				).
				Values(
					worker.GardenAddr,
					sq.Expr(expires),
					worker.ActiveContainers,
					resourceTypes,
					tags,
					worker.Platform,
					worker.BaggageclaimURL,
					worker.HTTPProxyURL,
					worker.HTTPSProxyURL,
					worker.NoProxy,
					worker.Name,
					worker.StartTime,
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
			Set("addr", worker.GardenAddr).
			Set("expires", sq.Expr(expires)).
			Set("active_containers", worker.ActiveContainers).
			Set("resource_types", resourceTypes).
			Set("tags", tags).
			Set("platform", worker.Platform).
			Set("baggageclaim_url", worker.BaggageclaimURL).
			Set("http_proxy_url", worker.HTTPProxyURL).
			Set("https_proxy_url", worker.HTTPSProxyURL).
			Set("no_proxy", worker.NoProxy).
			Set("name", worker.Name).
			Set("start_time", worker.StartTime).
			Set("state", string(workerState)).
			Where(sq.Eq{
				"name": worker.Name,
			}).
			RunWith(tx).
			Exec()
		if err != nil {
			return nil, err
		}
	}

	savedWorker := &Worker{
		Name:       worker.Name,
		GardenAddr: &worker.GardenAddr,
		State:      workerState,
	}

	workerBaseResourceTypeIDs := []int{}
	for _, resourceType := range worker.ResourceTypes {
		workerResourceType := WorkerResourceType{
			Worker:  savedWorker,
			Image:   resourceType.Image,
			Version: resourceType.Version,
			BaseResourceType: &BaseResourceType{
				Name: resourceType.Type,
			},
		}

		brt := BaseResourceType{
			Name: resourceType.Type,
		}

		ubrt, err := brt.FindOrCreate(tx)
		if err != nil {
			return nil, err
		}

		_, err = psql.Delete("worker_base_resource_types").
			Where(sq.Eq{
				"worker_name":           worker.Name,
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
		uwrt, err := workerResourceType.FindOrCreate(tx)
		if err != nil {
			return nil, err
		}

		workerBaseResourceTypeIDs = append(workerBaseResourceTypeIDs, uwrt.ID)
	}

	_, err = psql.Delete("worker_base_resource_types").
		Where(sq.Eq{
			"worker_name": worker.Name,
		}).
		Where(sq.NotEq{
			"id": workerBaseResourceTypeIDs,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return nil, err
	}

	return savedWorker, nil
}

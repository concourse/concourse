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
	SaveWorker(worker atc.Worker, ttl time.Duration) (*Worker, error)
	SaveTeamWorker(worker atc.Worker, team *Team, ttl time.Duration) (*Worker, error)
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
		name       string
		gardenAddr sql.NullString
		state      string

		baggageclaimURL string //TODO: should this become NullString just like gardenAddr? atm it's not allowed to be null
		httpProxyURL    sql.NullString
		httpsProxyURL   sql.NullString
		noProxy         sql.NullString

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
		&gardenAddr,
		&state,
		&baggageclaimURL,
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

	var addr *string
	if gardenAddr.Valid {
		addr = &gardenAddr.String
	}

	worker := Worker{
		Name:       name,
		GardenAddr: addr,
		State:      WorkerState(state),

		BaggageclaimURL:  baggageclaimURL,
		ActiveContainers: activeContainers,
		StartTime:        startTime,
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

func (f *workerFactory) StallWorker(name string) (*Worker, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var (
		workerName  string
		gardenAddr  sql.NullString
		workerState string
	)
	err = psql.Update("workers").
		SetMap(map[string]interface{}{
			"state":   string(WorkerStateStalled),
			"expires": nil,
			"addr":    nil,
		}).
		Where(sq.Eq{"name": name}).
		Suffix("RETURNING name, addr, state").
		RunWith(tx).
		QueryRow().
		Scan(&workerName, &gardenAddr, &workerState)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrWorkerNotPresent
		}
		return nil, err
	}

	var addr *string
	if gardenAddr.Valid {
		addr = &gardenAddr.String
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &Worker{
		Name:       workerName,
		GardenAddr: addr,
		State:      WorkerState(workerState),
	}, nil
}

func (f *workerFactory) StallUnresponsiveWorkers() ([]*Worker, error) {
	query, args, err := psql.Update("workers").
		SetMap(map[string]interface{}{
			"state":   string(WorkerStateStalled),
			"addr":    nil,
			"expires": nil,
		}).
		Where(sq.Eq{"state": string(WorkerStateRunning)}).
		Where(sq.Expr("expires < NOW()")).
		Suffix("RETURNING name, addr, state").
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
			name       string
			gardenAddr sql.NullString
			state      string
		)

		err = rows.Scan(&name, &gardenAddr, &state)
		if err != nil {
			return nil, err
		}

		var addr *string
		if gardenAddr.Valid {
			addr = &gardenAddr.String
		}

		workers = append(workers, &Worker{
			Name:       name,
			GardenAddr: addr,
			State:      WorkerState(state),
		})
	}

	return workers, nil
}

func (f *workerFactory) SaveWorker(worker atc.Worker, ttl time.Duration) (*Worker, error) {
	return f.saveWorker(worker, nil, ttl)
}

func (f *workerFactory) SaveTeamWorker(worker atc.Worker, team *Team, ttl time.Duration) (*Worker, error) {
	return f.saveWorker(worker, team, ttl)
}

func (f *workerFactory) saveWorker(worker atc.Worker, team *Team, ttl time.Duration) (*Worker, error) {
	resourceTypes, err := json.Marshal(worker.ResourceTypes)
	if err != nil {
		return nil, err
	}

	tags, err := json.Marshal(worker.Tags)
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

	var teamID *int
	if team != nil {
		teamID = &team.ID
	}

	var oldTeamID sql.NullInt64

	err = psql.Select("team_id").From("workers").Where(sq.Eq{
		"name": worker.Name,
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
					string(WorkerStateRunning),
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
			Set("state", string(WorkerStateRunning)).
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
		State:      WorkerStateRunning,
	}

	baseResourceTypeIDs := []int{}
	for _, resourceType := range worker.ResourceTypes {
		workerResourceType := WorkerResourceType{
			Worker:  savedWorker,
			Image:   resourceType.Image,
			Version: resourceType.Version,
			BaseResourceType: &BaseResourceType{
				Name: resourceType.Type,
			},
		}
		uwrt, err := workerResourceType.FindOrCreate(tx)
		if err != nil {
			return nil, err
		}

		baseResourceTypeIDs = append(baseResourceTypeIDs, uwrt.UsedBaseResourceType.ID)
	}

	_, err = psql.Delete("worker_base_resource_types").
		Where(sq.Eq{
			"worker_name": worker.Name,
		}).
		Where(sq.NotEq{
			"base_resource_type_id": baseResourceTypeIDs,
		}).
		RunWith(tx).
		Exec()

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return savedWorker, nil
}

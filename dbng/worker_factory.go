package dbng

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
)

//go:generate counterfeiter . WorkerFactory

type WorkerFactory interface {
	GetWorker(name string) (*Worker, bool, error)
	Workers() ([]*Worker, error)
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

	var (
		workerName  string
		workerAddr  sql.NullString
		workerState string
	)

	err = psql.Select("name, addr, state").
		From("workers").
		Where(sq.Eq{"name": name}).
		RunWith(tx).
		QueryRow().
		Scan(&workerName, &workerAddr, &workerState)
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

	var addr *string
	if workerAddr.Valid {
		addr = &workerAddr.String
	}

	return &Worker{
		Name:       workerName,
		State:      WorkerState(workerState),
		GardenAddr: addr,
	}, true, nil
}

func (f *workerFactory) Workers() ([]*Worker, error) {
	query, args, err := psql.Select("name, addr, state").
		From("workers").
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

	rows, err := psql.Update("workers").
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
		Set("team_id", teamID).
		Set("state", string(WorkerStateRunning)).
		Where(sq.Eq{
			"name": worker.Name,
			"addr": worker.GardenAddr,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return nil, err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return nil, err
	}

	if affected == 0 {
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

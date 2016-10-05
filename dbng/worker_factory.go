package dbng

import (
	"encoding/json"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
)

//go:generate counterfeiter . WorkerFactory

type WorkerFactory interface {
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
			).
			RunWith(tx).
			Exec()
		if err != nil {
			return nil, err
		}
	}

	savedWorker := &Worker{
		Name:       worker.Name,
		GardenAddr: worker.GardenAddr,
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

package dbng

import (
	"database/sql"
	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/nu7hatch/gouuid"
)

//go:generate counterfeiter . VolumeFactory

type VolumeFactory interface {
	GetTeamVolumes(teamID int) ([]CreatedVolume, error)

	CreateContainerVolume(int, *Worker, CreatingContainer, string) (CreatingVolume, error)
	FindContainerVolume(int, *Worker, CreatingContainer, string) (CreatingVolume, CreatedVolume, error)

	FindBaseResourceTypeVolume(int, *Worker, *UsedBaseResourceType) (CreatingVolume, CreatedVolume, error)
	CreateBaseResourceTypeVolume(int, *Worker, *UsedBaseResourceType) (CreatingVolume, error)

	FindResourceCacheVolume(*Worker, *UsedResourceCache) (CreatingVolume, CreatedVolume, error)
	FindResourceCacheInitializedVolume(*Worker, *UsedResourceCache) (CreatedVolume, bool, error)
	CreateResourceCacheVolume(*Worker, *UsedResourceCache) (CreatingVolume, error)

	FindVolumesForContainer(CreatedContainer) ([]CreatedVolume, error)
	GetOrphanedVolumes() ([]CreatedVolume, []DestroyingVolume, error)

	FindCreatedVolume(handle string) (CreatedVolume, bool, error)
}

type volumeFactory struct {
	conn Conn
}

func NewVolumeFactory(conn Conn) VolumeFactory {
	return &volumeFactory{
		conn: conn,
	}
}

func (factory *volumeFactory) GetTeamVolumes(teamID int) ([]CreatedVolume, error) {
	query, args, err := psql.Select("id, handle, state, worker_name, size_in_bytes", factory.typeSelector()).
		From("volumes").
		Where(sq.Or{
			sq.Eq{
				"team_id": teamID,
			},
			sq.Eq{
				"team_id": nil,
			},
		}).
		Where(sq.Eq{
			"state": "created",
		}).ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := factory.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	createdVolumes := []CreatedVolume{}

	for rows.Next() {
		var id int
		var handle string
		var state string
		var workerName string
		var bytes int64
		var volumeType VolumeType

		err = rows.Scan(&id, &handle, &state, &workerName, &bytes, &volumeType)
		if err != nil {
			return nil, err
		}

		createdVolumes = append(createdVolumes, &createdVolume{
			id:     id,
			handle: handle,
			typ:    volumeType,
			worker: &Worker{
				Name: workerName,
			},
			conn: factory.conn,
		})
	}

	return createdVolumes, nil
}

func (factory *volumeFactory) CreateResourceCacheVolume(worker *Worker, resourceCache *UsedResourceCache) (CreatingVolume, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	return factory.createVolume(
		tx,
		0,
		worker,
		map[string]interface{}{"resource_cache_id": resourceCache.ID},
		VolumeTypeResource,
	)
}

func (factory *volumeFactory) CreateBaseResourceTypeVolume(teamID int, worker *Worker, ubrt *UsedBaseResourceType) (CreatingVolume, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	return factory.createVolume(
		tx,
		teamID,
		worker,
		map[string]interface{}{
			"base_resource_type_id": ubrt.ID,
			"initialized":           true,
		},
		VolumeTypeResourceType,
	)
}

func (factory *volumeFactory) CreateContainerVolume(teamID int, worker *Worker, container CreatingContainer, mountPath string) (CreatingVolume, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	volume, err := factory.createVolume(
		tx,
		teamID,
		worker,
		map[string]interface{}{
			"container_id": container.ID(),
			"path":         mountPath,
			"initialized":  true,
		},
		VolumeTypeContainer,
	)
	if err != nil {
		return nil, err
	}

	volume.path = mountPath
	return volume, nil
}

func (factory *volumeFactory) FindVolumesForContainer(container CreatedContainer) ([]CreatedVolume, error) {
	query, args, err := psql.Select("v.id, v.handle, v.path, v.state, w.name, w.addr", factory.typeSelector()).
		From("volumes v").
		LeftJoin("workers w ON v.worker_name = w.name").
		Where(sq.Eq{
			"v.state":        VolumeStateCreated,
			"v.container_id": container.ID(),
		}).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := factory.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	createdVolumes := []CreatedVolume{}

	for rows.Next() {
		var id int
		var handle string
		var path sql.NullString
		var state string
		var workerName string
		var workerAddress sql.NullString
		var volumeType VolumeType

		err = rows.Scan(&id, &handle, &path, &state, &workerName, &workerAddress, &volumeType)
		if err != nil {
			return nil, err
		}

		var pathString string
		if path.Valid {
			pathString = path.String
		}

		worker := Worker{
			Name:       workerName,
			GardenAddr: nil,
		}
		if workerAddress.Valid {
			worker.GardenAddr = &workerAddress.String
		}

		createdVolumes = append(createdVolumes, &createdVolume{
			id:     id,
			handle: handle,
			typ:    volumeType,
			path:   pathString,
			worker: &worker,
			conn:   factory.conn,
		})
	}

	return createdVolumes, nil
}

func (factory *volumeFactory) FindContainerVolume(teamID int, worker *Worker, container CreatingContainer, mountPath string) (CreatingVolume, CreatedVolume, error) {
	return factory.findVolume(teamID, worker, map[string]interface{}{
		"v.container_id": container.ID(),
		"v.path":         mountPath,
	})
}

func (factory *volumeFactory) FindBaseResourceTypeVolume(teamID int, worker *Worker, ubrt *UsedBaseResourceType) (CreatingVolume, CreatedVolume, error) {
	return factory.findVolume(teamID, worker, map[string]interface{}{
		"v.base_resource_type_id": ubrt.ID,
	})
}

func (factory *volumeFactory) FindResourceCacheVolume(worker *Worker, resourceCache *UsedResourceCache) (CreatingVolume, CreatedVolume, error) {
	return factory.findVolume(0, worker, map[string]interface{}{
		"v.resource_cache_id": resourceCache.ID,
	})
}

func (factory *volumeFactory) FindResourceCacheInitializedVolume(worker *Worker, resourceCache *UsedResourceCache) (CreatedVolume, bool, error) {
	_, createdVolume, err := factory.findVolume(0, worker, map[string]interface{}{
		"v.resource_cache_id": resourceCache.ID,
		"v.initialized":       true,
	})
	if err != nil {
		return nil, false, err
	}

	if createdVolume == nil {
		return nil, false, nil
	}

	return createdVolume, true, nil
}

func (factory *volumeFactory) FindCreatedVolume(handle string) (CreatedVolume, bool, error) {
	_, createdVolume, err := factory.findVolume(0, nil, map[string]interface{}{
		"v.handle": handle,
	})
	if err != nil {
		return nil, false, err
	}

	if createdVolume == nil {
		return nil, false, nil
	}

	return createdVolume, true, nil
}

func (factory *volumeFactory) GetOrphanedVolumes() ([]CreatedVolume, []DestroyingVolume, error) {
	query, args, err := psql.Select("v.id, v.handle, v.path, v.state, w.name, w.addr, w.baggageclaim_url").
		From("volumes v").
		LeftJoin("workers w ON v.worker_name = w.name").
		Where(sq.Eq{
			"v.initialized":           true,
			"v.resource_cache_id":     nil,
			"v.base_resource_type_id": nil,
			"v.container_id":          nil,
		}).ToSql()
	if err != nil {
		return nil, nil, err
	}

	rows, err := factory.conn.Query(query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	createdVolumes := []CreatedVolume{}
	destroyingVolumes := []DestroyingVolume{}

	for rows.Next() {
		var id int
		var handle string
		var path sql.NullString
		var state string
		var workerName string
		var workerAddress sql.NullString
		var workerBaggageclaimURL sql.NullString

		err = rows.Scan(&id, &handle, &path, &state, &workerName, &workerAddress, &workerBaggageclaimURL)
		if err != nil {
			return nil, nil, err
		}

		var pathString string
		if path.Valid {
			pathString = path.String
		}

		var workerAddrString string
		if workerAddress.Valid {
			workerAddrString = workerAddress.String
		}

		var workerBaggageclaimURLString string
		if workerBaggageclaimURL.Valid {
			workerBaggageclaimURLString = workerBaggageclaimURL.String
		}

		switch state {
		case VolumeStateCreated:
			createdVolumes = append(createdVolumes, &createdVolume{
				id:     id,
				handle: handle,
				path:   pathString,
				worker: &Worker{
					Name:            workerName,
					GardenAddr:      &workerAddrString,
					BaggageclaimURL: &workerBaggageclaimURLString,
				},
				conn: factory.conn,
			})
		case VolumeStateDestroying:
			destroyingVolumes = append(destroyingVolumes, &destroyingVolume{
				id:     id,
				handle: handle,
				worker: &Worker{
					Name:            workerName,
					GardenAddr:      &workerAddrString,
					BaggageclaimURL: &workerBaggageclaimURLString,
				},
				conn: factory.conn,
			})
		}
	}

	return createdVolumes, destroyingVolumes, nil
}

// 1. open tx
// 2. lookup cache id
//   * if not found, create.
//     * if fails (unique violation; concurrent create), goto 1.
// 3. insert into volumes in 'initializing' state
//   * if fails (fkey violation; preexisting cache id was removed), goto 1.
// 4. commit tx

var ErrWorkerResourceTypeNotFound = errors.New("worker resource type no longer exists (stale?)")

// 1. open tx
// 2. lookup worker resource type id
//   * if not found, fail; worker must have new version or no longer supports type
// 3. insert into volumes in 'initializing' state
//   * if fails (fkey violation; worker type gone), fail for same reason as 2.
// 4. commit tx
func (factory *volumeFactory) createVolume(tx Tx, teamID int, worker *Worker, columns map[string]interface{}, volumeType VolumeType) (*creatingVolume, error) {
	var volumeID int
	handle, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	columnNames := []string{"worker_name", "handle"}
	columnValues := []interface{}{worker.Name, handle.String()}
	for name, value := range columns {
		columnNames = append(columnNames, name)
		columnValues = append(columnValues, value)
	}

	if teamID != 0 {
		columnNames = append(columnNames, "team_id")
		columnValues = append(columnValues, teamID)
	}

	err = psql.Insert("volumes").
		Columns(columnNames...). // hey, replace this with SetMap plz
		Values(columnValues...).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&volumeID)
	if err != nil {
		// TODO: explicitly handle fkey constraint on wrt id
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &creatingVolume{
		worker: worker,

		id:     volumeID,
		handle: handle.String(),
		typ:    volumeType,

		conn: factory.conn,
	}, nil
}

func (factory *volumeFactory) findVolume(teamID int, worker *Worker, columns map[string]interface{}) (CreatingVolume, CreatedVolume, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, nil, err
	}

	defer tx.Rollback()

	var id int
	var handle string
	var state string
	var workerName string
	var workerAddress string
	var path sql.NullString
	var volumeType VolumeType

	whereClause := sq.Eq{}
	if teamID != 0 {
		whereClause["v.team_id"] = teamID
	}
	if worker != nil {
		whereClause["v.worker_name"] = worker.Name
	}

	for name, value := range columns {
		whereClause[name] = value
	}

	err = psql.Select(
		"v.id, v.handle, v.state, v.path, w.name, w.addr",
		factory.typeSelector()).
		From("volumes v").
		LeftJoin("workers w ON v.worker_name = w.name").
		Where(whereClause).
		RunWith(tx).
		QueryRow().
		Scan(&id, &handle, &state, &path, &workerName, &workerAddress, &volumeType)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	err = tx.Commit()
	if err != nil {
		return nil, nil, err
	}

	var pathString string
	if path.Valid {
		pathString = path.String
	}

	switch state {
	case VolumeStateCreated:
		return nil, &createdVolume{
			id:     id,
			handle: handle,
			typ:    volumeType,
			path:   pathString,
			worker: &Worker{
				Name:       workerName,
				GardenAddr: &workerAddress,
			},
			conn: factory.conn,
		}, nil
	case VolumeStateCreating:
		return &creatingVolume{
			id:     id,
			handle: handle,
			typ:    volumeType,
			path:   pathString,
			worker: &Worker{
				Name:       workerName,
				GardenAddr: &workerAddress,
			},
			conn: factory.conn,
		}, nil, nil
	}

	return nil, nil, nil
}

func (factory *volumeFactory) typeSelector() string {
	return `case when container_id is not NULL then 'container'
	  when resource_cache_id is not NULL then 'resource'
		when base_resource_type_id is not NULL then 'resource-type'
		else 'unknown'
	end`
}

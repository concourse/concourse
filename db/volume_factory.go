package db

import (
	"database/sql"
	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/nu7hatch/gouuid"
)

//go:generate counterfeiter . VolumeFactory

type VolumeFactory interface {
	GetTeamVolumes(teamID int) ([]CreatedVolume, error)

	CreateContainerVolume(int, string, CreatingContainer, string) (CreatingVolume, error)
	FindContainerVolume(int, string, CreatingContainer, string) (CreatingVolume, CreatedVolume, error)

	FindBaseResourceTypeVolume(int, *UsedWorkerBaseResourceType) (CreatingVolume, CreatedVolume, error)
	CreateBaseResourceTypeVolume(int, *UsedWorkerBaseResourceType) (CreatingVolume, error)

	FindResourceCacheVolume(string, *UsedResourceCache) (CreatedVolume, bool, error)

	FindTaskCacheVolume(teamID int, uwtc *UsedWorkerTaskCache) (CreatingVolume, CreatedVolume, error)
	CreateTaskCacheVolume(teamID int, uwtc *UsedWorkerTaskCache) (CreatingVolume, error)

	FindVolumesForContainer(CreatedContainer) ([]CreatedVolume, error)
	GetOrphanedVolumes() ([]CreatedVolume, []DestroyingVolume, error)

	GetFailedVolumes() ([]FailedVolume, error)

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
	query, args, err := psql.Select(volumeColumns...).
		From("volumes v").
		LeftJoin("workers w ON v.worker_name = w.name").
		LeftJoin("containers c ON v.container_id = c.id").
		LeftJoin("volumes pv ON v.parent_id = pv.id").
		LeftJoin("worker_resource_caches wrc ON wrc.id = v.worker_resource_cache_id").
		Where(sq.Or{
			sq.Eq{
				"v.team_id": teamID,
			},
			sq.Eq{
				"v.team_id": nil,
			},
		}).
		Where(sq.Eq{
			"v.state": "created",
		}).ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := factory.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	createdVolumes := []CreatedVolume{}

	for rows.Next() {
		_, createdVolume, _, _, err := scanVolume(rows, factory.conn)
		if err != nil {
			return nil, err
		}

		createdVolumes = append(createdVolumes, createdVolume)
	}

	return createdVolumes, nil
}

func (factory *volumeFactory) CreateBaseResourceTypeVolume(teamID int, uwbrt *UsedWorkerBaseResourceType) (CreatingVolume, error) {
	volume, err := factory.createVolume(
		teamID,
		uwbrt.WorkerName,
		map[string]interface{}{
			"worker_base_resource_type_id": uwbrt.ID,
		},
		VolumeTypeResourceType,
	)
	if err != nil {
		return nil, err
	}

	volume.workerBaseResourceTypeID = uwbrt.ID
	return volume, nil
}

func (factory *volumeFactory) CreateContainerVolume(teamID int, workerName string, container CreatingContainer, mountPath string) (CreatingVolume, error) {
	volume, err := factory.createVolume(
		teamID,
		workerName,
		map[string]interface{}{
			"container_id": container.ID(),
			"path":         mountPath,
		},
		VolumeTypeContainer,
	)
	if err != nil {
		return nil, err
	}

	volume.path = mountPath
	volume.containerHandle = container.Handle()
	return volume, nil
}

func (factory *volumeFactory) FindVolumesForContainer(container CreatedContainer) ([]CreatedVolume, error) {
	query, args, err := psql.Select(volumeColumns...).
		From("volumes v").
		LeftJoin("workers w ON v.worker_name = w.name").
		LeftJoin("containers c ON v.container_id = c.id").
		LeftJoin("volumes pv ON v.parent_id = pv.id").
		LeftJoin("worker_resource_caches wrc ON wrc.id = v.worker_resource_cache_id").
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
	defer Close(rows)

	createdVolumes := []CreatedVolume{}

	for rows.Next() {
		_, createdVolume, _, _, err := scanVolume(rows, factory.conn)
		if err != nil {
			return nil, err
		}

		createdVolumes = append(createdVolumes, createdVolume)
	}

	return createdVolumes, nil
}

func (factory *volumeFactory) FindContainerVolume(teamID int, workerName string, container CreatingContainer, mountPath string) (CreatingVolume, CreatedVolume, error) {
	return factory.findVolume(teamID, workerName, map[string]interface{}{
		"v.container_id": container.ID(),
		"v.path":         mountPath,
	})
}

func (factory *volumeFactory) FindBaseResourceTypeVolume(teamID int, uwbrt *UsedWorkerBaseResourceType) (CreatingVolume, CreatedVolume, error) {
	return factory.findVolume(teamID, uwbrt.WorkerName, map[string]interface{}{
		"v.worker_base_resource_type_id": uwbrt.ID,
	})
}

func (factory *volumeFactory) FindTaskCacheVolume(teamID int, uwtc *UsedWorkerTaskCache) (CreatingVolume, CreatedVolume, error) {
	return factory.findVolume(teamID, uwtc.WorkerName, map[string]interface{}{
		"v.worker_task_cache_id": uwtc.ID,
	})
}

func (factory *volumeFactory) CreateTaskCacheVolume(teamID int, uwtc *UsedWorkerTaskCache) (CreatingVolume, error) {
	volume, err := factory.createVolume(
		teamID,
		uwtc.WorkerName,
		map[string]interface{}{
			"worker_task_cache_id": uwtc.ID,
		},
		VolumeTypeTaskCache,
	)
	if err != nil {
		return nil, err
	}

	volume.workerTaskCacheID = uwtc.ID
	return volume, nil
}

func (factory *volumeFactory) FindResourceCacheVolume(workerName string, resourceCache *UsedResourceCache) (CreatedVolume, bool, error) {
	workerResourceCache, found, err := WorkerResourceCache{
		WorkerName:    workerName,
		ResourceCache: resourceCache,
	}.Find(factory.conn)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	_, createdVolume, err := factory.findVolume(0, workerName, map[string]interface{}{
		"v.worker_resource_cache_id": workerResourceCache.ID,
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
	_, createdVolume, err := factory.findVolume(0, "", map[string]interface{}{
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
	query, args, err := psql.Select(volumeColumns...).
		From("volumes v").
		LeftJoin("workers w ON v.worker_name = w.name").
		LeftJoin("containers c ON v.container_id = c.id").
		LeftJoin("volumes pv ON v.parent_id = pv.id").
		LeftJoin("worker_resource_caches wrc ON wrc.id = v.worker_resource_cache_id").
		Where(sq.Eq{
			"v.worker_resource_cache_id":     nil,
			"v.worker_base_resource_type_id": nil,
			"v.container_id":                 nil,
			"v.worker_task_cache_id":         nil,
		}).
		Where(sq.Or{
			sq.Eq{"v.state": string(VolumeStateCreated)},
			sq.Eq{"v.state": string(VolumeStateDestroying)},
		}).
		Where(sq.Or{
			sq.Eq{"w.state": string(WorkerStateRunning)},
			sq.Eq{"w.state": string(WorkerStateLanding)},
			sq.Eq{"w.state": string(WorkerStateRetiring)},
		}).
		ToSql()
	if err != nil {
		return nil, nil, err
	}

	rows, err := factory.conn.Query(query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer Close(rows)

	createdVolumes := []CreatedVolume{}
	destroyingVolumes := []DestroyingVolume{}

	for rows.Next() {
		_, createdVolume, destroyingVolume, _, err := scanVolume(rows, factory.conn)

		if err != nil {
			return nil, nil, err
		}

		if createdVolume != nil {
			createdVolumes = append(createdVolumes, createdVolume)
		}

		if destroyingVolume != nil {
			destroyingVolumes = append(destroyingVolumes, destroyingVolume)
		}
	}

	return createdVolumes, destroyingVolumes, nil
}

func (factory *volumeFactory) GetFailedVolumes() ([]FailedVolume, error) {
	query, args, err := psql.Select(volumeColumns...).
		From("volumes v").
		LeftJoin("workers w ON v.worker_name = w.name").
		LeftJoin("containers c ON v.container_id = c.id").
		LeftJoin("volumes pv ON v.parent_id = pv.id").
		LeftJoin("worker_resource_caches wrc ON wrc.id = v.worker_resource_cache_id").
		Where(sq.Eq{
			"v.state": string(VolumeStateFailed),
		}).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := factory.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	failedVolumes := []FailedVolume{}

	for rows.Next() {
		_, _, _, failedVolume, err := scanVolume(rows, factory.conn)

		if err != nil {
			return nil, err
		}

		if failedVolume != nil {
			failedVolumes = append(failedVolumes, failedVolume)
		}
	}

	return failedVolumes, nil
}

var ErrWorkerResourceTypeNotFound = errors.New("worker resource type no longer exists (stale?)")

// 1. open tx
// 2. lookup worker resource type id
//   * if not found, fail; worker must have new version or no longer supports type
// 3. insert into volumes in 'initializing' state
//   * if fails (fkey violation; worker type gone), fail for same reason as 2.
// 4. commit tx
func (factory *volumeFactory) createVolume(
	teamID int,
	workerName string,
	columns map[string]interface{},
	volumeType VolumeType,
) (*creatingVolume, error) {
	var volumeID int
	handle, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	columnNames := []string{"worker_name", "handle"}
	columnValues := []interface{}{workerName, handle.String()}
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
		RunWith(factory.conn).
		QueryRow().
		Scan(&volumeID)
	if err != nil {
		return nil, err
	}

	return &creatingVolume{
		workerName: workerName,

		id:     volumeID,
		handle: handle.String(),
		typ:    volumeType,
		teamID: teamID,

		conn: factory.conn,
	}, nil
}

func (factory *volumeFactory) findVolume(teamID int, workerName string, columns map[string]interface{}) (CreatingVolume, CreatedVolume, error) {
	whereClause := sq.Eq{}
	if teamID != 0 {
		whereClause["v.team_id"] = teamID
	}
	if workerName != "" {
		whereClause["v.worker_name"] = workerName
	}

	for name, value := range columns {
		whereClause[name] = value
	}

	row := psql.Select(volumeColumns...).
		From("volumes v").
		LeftJoin("workers w ON v.worker_name = w.name").
		LeftJoin("containers c ON v.container_id = c.id").
		LeftJoin("volumes pv ON v.parent_id = pv.id").
		LeftJoin("worker_resource_caches wrc ON wrc.id = v.worker_resource_cache_id").
		Where(whereClause).
		RunWith(factory.conn).
		QueryRow()
	creatingVolume, createdVolume, _, _, err := scanVolume(row, factory.conn)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	return creatingVolume, createdVolume, nil
}

var volumeColumns = []string{
	"v.id",
	"v.handle",
	"v.state",
	"w.name",
	"v.path",
	"c.handle",
	"pv.handle",
	"v.team_id",
	"wrc.resource_cache_id",
	"v.worker_base_resource_type_id",
	"v.worker_task_cache_id",
	`case
	when v.worker_base_resource_type_id is not NULL then 'resource-type'
	when v.worker_resource_cache_id is not NULL then 'resource'
	when v.container_id is not NULL then 'container'
	when v.worker_task_cache_id is not NULL then 'task-cache'
	else 'unknown'
end`,
}

func scanVolume(row sq.RowScanner, conn Conn) (CreatingVolume, CreatedVolume, DestroyingVolume, FailedVolume, error) {
	var id int
	var handle string
	var state string
	var workerName string
	var sqPath sql.NullString
	var sqContainerHandle sql.NullString
	var sqParentHandle sql.NullString
	var sqTeamID sql.NullInt64
	var sqResourceCacheID sql.NullInt64
	var sqWorkerBaseResourceTypeID sql.NullInt64
	var sqWorkerTaskCacheID sql.NullInt64
	var volumeType VolumeType

	err := row.Scan(
		&id,
		&handle,
		&state,
		&workerName,
		&sqPath,
		&sqContainerHandle,
		&sqParentHandle,
		&sqTeamID,
		&sqResourceCacheID,
		&sqWorkerBaseResourceTypeID,
		&sqWorkerTaskCacheID,
		&volumeType,
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	var path string
	if sqPath.Valid {
		path = sqPath.String
	}

	var containerHandle string
	if sqContainerHandle.Valid {
		containerHandle = sqContainerHandle.String
	}

	var parentHandle string
	if sqParentHandle.Valid {
		parentHandle = sqParentHandle.String
	}

	var teamID int
	if sqTeamID.Valid {
		teamID = int(sqTeamID.Int64)
	}

	var resourceCacheID int
	if sqResourceCacheID.Valid {
		resourceCacheID = int(sqResourceCacheID.Int64)
	}

	var workerBaseResourceTypeID int
	if sqWorkerBaseResourceTypeID.Valid {
		workerBaseResourceTypeID = int(sqWorkerBaseResourceTypeID.Int64)
	}

	var workerTaskCacheID int
	if sqWorkerTaskCacheID.Valid {
		workerTaskCacheID = int(sqWorkerTaskCacheID.Int64)
	}

	switch VolumeState(state) {
	case VolumeStateCreated:
		return nil, &createdVolume{
			id:                       id,
			handle:                   handle,
			typ:                      volumeType,
			path:                     path,
			teamID:                   teamID,
			workerName:               workerName,
			containerHandle:          containerHandle,
			parentHandle:             parentHandle,
			resourceCacheID:          resourceCacheID,
			workerBaseResourceTypeID: workerBaseResourceTypeID,
			workerTaskCacheID:        workerTaskCacheID,
			conn:                     conn,
		}, nil, nil, nil
	case VolumeStateCreating:
		return &creatingVolume{
			id:                       id,
			handle:                   handle,
			typ:                      volumeType,
			path:                     path,
			teamID:                   teamID,
			workerName:               workerName,
			containerHandle:          containerHandle,
			parentHandle:             parentHandle,
			resourceCacheID:          resourceCacheID,
			workerBaseResourceTypeID: workerBaseResourceTypeID,
			workerTaskCacheID:        workerTaskCacheID,
			conn:                     conn,
		}, nil, nil, nil, nil
	case VolumeStateDestroying:
		return nil, nil, &destroyingVolume{
			id:         id,
			handle:     handle,
			workerName: workerName,
			conn:       conn,
		}, nil, nil
	case VolumeStateFailed:
		return nil, nil, nil, &failedVolume{
			id:         id,
			handle:     handle,
			workerName: workerName,
			conn:       conn,
		}, nil
	}

	return nil, nil, nil, nil, nil
}

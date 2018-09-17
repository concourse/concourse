package db

import (
	"database/sql"
	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/nu7hatch/gouuid"
)

//go:generate counterfeiter . VolumeRepository

type VolumeRepository interface {
	GetTeamVolumes(teamID int) ([]CreatedVolume, error)

	CreateContainerVolume(int, string, CreatingContainer, string) (CreatingVolume, error)
	FindContainerVolume(int, string, CreatingContainer, string) (CreatingVolume, CreatedVolume, error)

	FindBaseResourceTypeVolume(int, *UsedWorkerBaseResourceType) (CreatingVolume, CreatedVolume, error)
	CreateBaseResourceTypeVolume(int, *UsedWorkerBaseResourceType) (CreatingVolume, error)

	FindResourceCacheVolume(string, UsedResourceCache) (CreatedVolume, bool, error)

	FindTaskCacheVolume(teamID int, uwtc *UsedWorkerTaskCache) (CreatingVolume, CreatedVolume, error)
	CreateTaskCacheVolume(teamID int, uwtc *UsedWorkerTaskCache) (CreatingVolume, error)

	FindResourceCertsVolume(workerName string, uwrc *UsedWorkerResourceCerts) (CreatingVolume, CreatedVolume, error)
	CreateResourceCertsVolume(workerName string, uwrc *UsedWorkerResourceCerts) (CreatingVolume, error)

	FindVolumesForContainer(CreatedContainer) ([]CreatedVolume, error)
	GetOrphanedVolumes() ([]CreatedVolume, error)

	DestroyFailedVolumes() (int, error)

	GetDestroyingVolumes(workerName string) ([]string, error)

	FindCreatedVolume(handle string) (CreatedVolume, bool, error)

	RemoveDestroyingVolumes(workerName string, handles []string) (int, error)
}

type volumeRepository struct {
	conn Conn
}

func NewVolumeRepository(conn Conn) VolumeRepository {
	return &volumeRepository{
		conn: conn,
	}
}

func (repository *volumeRepository) RemoveDestroyingVolumes(workerName string, handles []string) (int, error) {
	rows, err := psql.Delete("volumes").
		Where(
			sq.And{
				sq.Eq{
					"worker_name": workerName,
				},
				sq.NotEq{
					"handle": handles,
				},
				sq.Eq{
					"state": VolumeStateDestroying,
				},
			},
		).RunWith(repository.conn).
		Exec()

	if err != nil {
		return 0, err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(affected), nil
}

func (repository *volumeRepository) GetTeamVolumes(teamID int) ([]CreatedVolume, error) {
	query, args, err := psql.Select(volumeColumns...).
		From("volumes v").
		LeftJoin("workers w ON v.worker_name = w.name").
		LeftJoin("containers c ON v.container_id = c.id").
		LeftJoin("volumes pv ON v.parent_id = pv.id").
		LeftJoin("worker_resource_caches wrc ON wrc.id = v.worker_resource_cache_id").
		LeftJoin("worker_resource_certs  certs ON certs.id = v.worker_resource_certs_id").
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

	rows, err := repository.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	createdVolumes := []CreatedVolume{}

	for rows.Next() {
		_, createdVolume, _, _, err := scanVolume(rows, repository.conn)
		if err != nil {
			return nil, err
		}

		createdVolumes = append(createdVolumes, createdVolume)
	}

	return createdVolumes, nil
}

func (repository *volumeRepository) CreateBaseResourceTypeVolume(teamID int, uwbrt *UsedWorkerBaseResourceType) (CreatingVolume, error) {
	volume, err := repository.createVolume(
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

func (repository *volumeRepository) CreateContainerVolume(teamID int, workerName string, container CreatingContainer, mountPath string) (CreatingVolume, error) {
	volume, err := repository.createVolume(
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

func (repository *volumeRepository) FindVolumesForContainer(container CreatedContainer) ([]CreatedVolume, error) {
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

	rows, err := repository.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	createdVolumes := []CreatedVolume{}

	for rows.Next() {
		_, createdVolume, _, _, err := scanVolume(rows, repository.conn)
		if err != nil {
			return nil, err
		}

		createdVolumes = append(createdVolumes, createdVolume)
	}

	return createdVolumes, nil
}

func (repository *volumeRepository) FindContainerVolume(teamID int, workerName string, container CreatingContainer, mountPath string) (CreatingVolume, CreatedVolume, error) {
	return repository.findVolume(teamID, workerName, map[string]interface{}{
		"v.container_id": container.ID(),
		"v.path":         mountPath,
	})
}

func (repository *volumeRepository) FindBaseResourceTypeVolume(teamID int, uwbrt *UsedWorkerBaseResourceType) (CreatingVolume, CreatedVolume, error) {
	return repository.findVolume(teamID, uwbrt.WorkerName, map[string]interface{}{
		"v.worker_base_resource_type_id": uwbrt.ID,
	})
}

func (repository *volumeRepository) FindTaskCacheVolume(teamID int, uwtc *UsedWorkerTaskCache) (CreatingVolume, CreatedVolume, error) {
	return repository.findVolume(teamID, uwtc.WorkerName, map[string]interface{}{
		"v.worker_task_cache_id": uwtc.ID,
	})
}

func (repository *volumeRepository) CreateTaskCacheVolume(teamID int, uwtc *UsedWorkerTaskCache) (CreatingVolume, error) {
	volume, err := repository.createVolume(
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

func (repository *volumeRepository) FindResourceCertsVolume(workerName string, uwrc *UsedWorkerResourceCerts) (CreatingVolume, CreatedVolume, error) {
	return repository.findVolume(0, workerName, map[string]interface{}{
		"v.worker_resource_certs_id": uwrc.ID,
	})
}

func (repository *volumeRepository) CreateResourceCertsVolume(workerName string, uwrc *UsedWorkerResourceCerts) (CreatingVolume, error) {
	volume, err := repository.createVolume(
		0,
		workerName,
		map[string]interface{}{
			"worker_resource_certs_id": uwrc.ID,
		},
		VolumeTypeResourceCerts,
	)
	if err != nil {
		return nil, err
	}

	return volume, nil
}

func (repository *volumeRepository) FindResourceCacheVolume(workerName string, resourceCache UsedResourceCache) (CreatedVolume, bool, error) {
	workerResourceCache, found, err := WorkerResourceCache{
		WorkerName:    workerName,
		ResourceCache: resourceCache,
	}.Find(repository.conn)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	_, createdVolume, err := repository.findVolume(0, workerName, map[string]interface{}{
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

func (repository *volumeRepository) FindCreatedVolume(handle string) (CreatedVolume, bool, error) {
	_, createdVolume, err := repository.findVolume(0, "", map[string]interface{}{
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

func (repository *volumeRepository) GetOrphanedVolumes() ([]CreatedVolume, error) {
	query, args, err := psql.Select(volumeColumns...).
		From("volumes v").
		LeftJoin("workers w ON v.worker_name = w.name").
		LeftJoin("containers c ON v.container_id = c.id").
		LeftJoin("volumes pv ON v.parent_id = pv.id").
		LeftJoin("worker_resource_caches wrc ON wrc.id = v.worker_resource_cache_id").
		Where(
			sq.Or{
				sq.Eq{
					"v.worker_resource_cache_id":     nil,
					"v.worker_base_resource_type_id": nil,
					"v.container_id":                 nil,
					"v.worker_task_cache_id":         nil,
					"v.worker_resource_certs_id":     nil,
				},
				sq.And{
					sq.NotEq{
						"v.worker_base_resource_type_id": nil,
					},
					sq.Eq{
						"v.worker_resource_cache_id": nil,
						"v.team_id":                  nil,
						"v.container_id":             nil,
						"v.worker_task_cache_id":     nil,
						"v.worker_resource_certs_id": nil,
					},
				},
			},
		).
		Where(sq.Eq{"v.state": string(VolumeStateCreated)}).
		Where(sq.Or{
			sq.Eq{"w.state": string(WorkerStateRunning)},
			sq.Eq{"w.state": string(WorkerStateLanding)},
			sq.Eq{"w.state": string(WorkerStateRetiring)},
		}).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := repository.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	createdVolumes := []CreatedVolume{}

	for rows.Next() {
		_, createdVolume, _, _, err := scanVolume(rows, repository.conn)

		if err != nil {
			return nil, err
		}

		if createdVolume != nil {
			createdVolumes = append(createdVolumes, createdVolume)
		}

	}

	return createdVolumes, nil
}

func (repository *volumeRepository) DestroyFailedVolumes() (int, error) {
	queryId, args, err := psql.Select("v.id").
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
		return 0, err
	}

	rows, err := sq.Delete("volumes").
		Where("id IN ("+queryId+")", args...).
		RunWith(repository.conn).
		Exec()
	if err != nil {
		return 0, err
	}

	failedVolumeLen, err := rows.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(failedVolumeLen), nil
}

func (repository *volumeRepository) GetDestroyingVolumes(workerName string) ([]string, error) {
	destroyingHandles := []string{}

	query, args, err := psql.Select("handle").
		From("volumes").
		Where(sq.Eq{
			"state":       string(VolumeStateDestroying),
			"worker_name": workerName,
		}).
		ToSql()

	if err != nil {
		return nil, err
	}

	rows, err := repository.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	for rows.Next() {
		var handle = "handle"
		columns := []interface{}{&handle}

		err = rows.Scan(columns...)
		if err != nil {
			return nil, err
		}

		destroyingHandles = append(destroyingHandles, handle)
	}

	return destroyingHandles, nil
}

var ErrWorkerResourceTypeNotFound = errors.New("worker resource type no longer exists (stale?)")

// 1. open tx
// 2. lookup worker resource type id
//   * if not found, fail; worker must have new version or no longer supports type
// 3. insert into volumes in 'initializing' state
//   * if fails (fkey violation; worker type gone), fail for same reason as 2.
// 4. commit tx
func (repository *volumeRepository) createVolume(
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
		RunWith(repository.conn).
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

		conn: repository.conn,
	}, nil
}

func (repository *volumeRepository) findVolume(teamID int, workerName string, columns map[string]interface{}) (CreatingVolume, CreatedVolume, error) {
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
		RunWith(repository.conn).
		QueryRow()
	creatingVolume, createdVolume, _, _, err := scanVolume(row, repository.conn)
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
	"v.worker_resource_certs_id",
	`case
	when v.worker_base_resource_type_id is not NULL then 'resource-type'
	when v.worker_resource_cache_id is not NULL then 'resource'
	when v.container_id is not NULL then 'container'
	when v.worker_task_cache_id is not NULL then 'task-cache'
	when v.worker_resource_certs_id is not NULL then 'resource-certs'
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
	var sqWorkerResourceCertsID sql.NullInt64
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
		&sqWorkerResourceCertsID,
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

	var workerResourceCertsID int
	if sqWorkerResourceCertsID.Valid {
		workerResourceCertsID = int(sqWorkerResourceCertsID.Int64)
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
			workerResourceCertsID:    workerResourceCertsID,
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
			workerResourceCertsID:    workerResourceCertsID,
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

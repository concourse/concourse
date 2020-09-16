package db

import (
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	uuid "github.com/nu7hatch/gouuid"
)

//go:generate counterfeiter . VolumeRepository

type VolumeRepository interface {
	GetTeamVolumes(teamID int) ([]CreatedVolume, error)

	CreateContainerVolume(teamID int, workerName string, container CreatingContainer, mountPath string) (CreatingVolume, error)
	FindContainerVolume(teamID int, workerName string, container CreatingContainer, mountPath string) (CreatingVolume, CreatedVolume, error)

	FindBaseResourceTypeVolume(*UsedWorkerBaseResourceType) (CreatingVolume, CreatedVolume, error)
	CreateBaseResourceTypeVolume(*UsedWorkerBaseResourceType) (CreatingVolume, error)

	FindResourceCacheVolume(workerName string, resourceCache UsedResourceCache) (CreatedVolume, bool, error)

	FindTaskCacheVolume(teamID int, workerName string, taskCache UsedTaskCache) (CreatedVolume, bool, error)
	CreateTaskCacheVolume(teamID int, uwtc *UsedWorkerTaskCache) (CreatingVolume, error)

	FindResourceCertsVolume(workerName string, uwrc *UsedWorkerResourceCerts) (CreatingVolume, CreatedVolume, error)
	CreateResourceCertsVolume(workerName string, uwrc *UsedWorkerResourceCerts) (CreatingVolume, error)

	FindVolumesForContainer(container CreatedContainer) ([]CreatedVolume, error)
	GetOrphanedVolumes() ([]CreatedVolume, error)

	DestroyFailedVolumes() (count int, err error)

	GetDestroyingVolumes(workerName string) ([]string, error)

	CreateVolume(int, string, VolumeType) (CreatingVolume, error)
	FindCreatedVolume(handle string) (CreatedVolume, bool, error)

	RemoveDestroyingVolumes(workerName string, handles []string) (int, error)

	UpdateVolumesMissingSince(workerName string, handles []string) error
	RemoveMissingVolumes(gracePeriod time.Duration) (removed int, err error)

	DestroyUnknownVolumes(workerName string, handles []string) (int, error)
}

const noTeam = 0

type volumeRepository struct {
	conn Conn
}

func NewVolumeRepository(conn Conn) VolumeRepository {
	return &volumeRepository{
		conn: conn,
	}
}

func (repository *volumeRepository) queryVolumeHandles(tx Tx, cond sq.Eq) ([]string, error) {
	query, args, err := psql.Select("handle").From("volumes").Where(cond).ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(query, args...)
	if err != nil {
		return nil, err
	}

	defer Close(rows)

	var handles []string

	for rows.Next() {
		var handle = "handle"
		columns := []interface{}{&handle}

		err = rows.Scan(columns...)
		if err != nil {
			return nil, err
		}
		handles = append(handles, handle)
	}

	return handles, nil
}

func (repository *volumeRepository) UpdateVolumesMissingSince(workerName string, reportedHandles []string) error {
	// clear out missing_since for reported volumes
	query, args, err := psql.Update("volumes").
		Set("missing_since", nil).
		Where(sq.And{
			sq.Eq{"handle": reportedHandles},
			sq.NotEq{"missing_since": nil},
		},
		).ToSql()
	if err != nil {
		return err
	}

	tx, err := repository.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	_, err = tx.Exec(query, args...)
	if err != nil {
		return err
	}

	dbHandles, err := repository.queryVolumeHandles(
		tx,
		sq.Eq{
			"worker_name":   workerName,
			"missing_since": nil,
		})
	if err != nil {
		return err
	}

	handles := diff(dbHandles, reportedHandles)

	query, args, err = psql.Update("volumes").
		Set("missing_since", sq.Expr("now()")).
		Where(sq.And{
			sq.Eq{"handle": handles},
			sq.NotEq{"state": VolumeStateCreating},
		}).ToSql()
	if err != nil {
		return err
	}

	_, err = tx.Exec(query, args...)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Removes any volumes that exist in the database but are missing on the worker
// for over the designated grace time period.
func (repository *volumeRepository) RemoveMissingVolumes(gracePeriod time.Duration) (int, error) {
	tx, err := repository.conn.Begin()
	if err != nil {
		return 0, err
	}

	defer tx.Rollback()

	// Setting the foreign key constraint to deferred, meaning that the foreign
	// key constraint will not be executed until the end of the transaction. This
	// allows the gc query to remove any parent volumes as long as the child
	// volume that references it is also removed within the same transaction.
	_, err = tx.Exec("SET CONSTRAINTS volumes_parent_id_fkey DEFERRED")
	if err != nil {
		return 0, err
	}

	result, err := tx.Exec(`
	WITH RECURSIVE missing(id) AS (
		SELECT id FROM volumes WHERE missing_since IS NOT NULL and NOW() - missing_since > $1 AND state IN ($2, $3)
	UNION ALL
		SELECT v.id FROM missing m, volumes v WHERE v.parent_id = m.id
	)
	DELETE FROM volumes v USING missing m WHERE m.id = v.id`, fmt.Sprintf("%.0f seconds", gracePeriod.Seconds()), VolumeStateCreated, VolumeStateFailed)
	if err != nil {
		return 0, err
	}

	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(affected), nil
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
			"v.state": VolumeStateCreated,
		}).ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := repository.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	var createdVolumes []CreatedVolume

	for rows.Next() {
		_, createdVolume, _, _, err := scanVolume(rows, repository.conn)
		if err != nil {
			return nil, err
		}

		createdVolumes = append(createdVolumes, createdVolume)
	}

	return createdVolumes, nil
}

func (repository *volumeRepository) CreateBaseResourceTypeVolume(uwbrt *UsedWorkerBaseResourceType) (CreatingVolume, error) {
	volume, err := repository.createVolume(
		noTeam,
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

func (repository *volumeRepository) CreateVolume(teamID int, workerName string, volumeType VolumeType) (CreatingVolume, error) {
	volume, err := repository.createVolume(
		0,
		workerName,
		map[string]interface{}{
			"team_id": teamID,
		},
		volumeType,
	)
	if err != nil {
		return nil, err
	}

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

	var createdVolumes []CreatedVolume

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

func (repository *volumeRepository) FindBaseResourceTypeVolume(uwbrt *UsedWorkerBaseResourceType) (CreatingVolume, CreatedVolume, error) {
	return repository.findVolume(0, uwbrt.WorkerName, map[string]interface{}{
		"v.worker_base_resource_type_id": uwbrt.ID,
	})
}

func (repository *volumeRepository) FindTaskCacheVolume(teamID int, workerName string, taskCache UsedTaskCache) (CreatedVolume, bool, error) {
	usedWorkerTaskCache, found, err := WorkerTaskCache{
		WorkerName: workerName,
		TaskCache:  taskCache,
	}.find(repository.conn)

	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	_, createdVolume, err := repository.findVolume(teamID, workerName, map[string]interface{}{
		"v.worker_task_cache_id": usedWorkerTaskCache.ID,
	})
	if err != nil {
		return nil, false, err
	}

	if createdVolume == nil {
		return nil, false, nil
	}

	return createdVolume, true, nil
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
		noTeam,
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
	_, createdVolume, err := getVolume(repository.conn, map[string]interface{}{
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
			sq.Eq{
				"v.worker_resource_cache_id":     nil,
				"v.worker_base_resource_type_id": nil,
				"v.container_id":                 nil,
				"v.worker_task_cache_id":         nil,
				"v.worker_resource_certs_id":     nil,
				"v.worker_artifact_id":           nil,
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

	var createdVolumes []CreatedVolume

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
	tx, err := repository.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	volumes, err := repository.queryVolumeHandles(
		tx,
		sq.Eq{
			"state":       string(VolumeStateDestroying),
			"worker_name": workerName,
		},
	)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return volumes, nil
}

func (repository *volumeRepository) DestroyUnknownVolumes(workerName string, reportedHandles []string) (int, error) {
	tx, err := repository.conn.Begin()
	if err != nil {
		return 0, err
	}

	defer Rollback(tx)
	dbHandles, err := repository.queryVolumeHandles(tx, sq.Eq{
		"worker_name": workerName,
	})
	if err != nil {
		return 0, err
	}

	unknownHandles := diff(reportedHandles, dbHandles)

	if len(unknownHandles) == 0 {
		return 0, nil
	}

	insertBuilder := psql.Insert("volumes").Columns(
		"handle",
		"worker_name",
		"state",
	)

	for _, unknownHandle := range unknownHandles {
		insertBuilder = insertBuilder.Values(
			unknownHandle,
			workerName,
			VolumeStateDestroying,
		)
	}

	_, err = insertBuilder.RunWith(tx).Exec()
	if err != nil {
		return 0, err
	}

	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	return len(unknownHandles), nil
}

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

	return getVolume(repository.conn, whereClause)
}

func getVolume(conn Conn, where map[string]interface{}) (CreatingVolume, CreatedVolume, error) {
	row := psql.Select(volumeColumns...).
		From("volumes v").
		LeftJoin("workers w ON v.worker_name = w.name").
		LeftJoin("containers c ON v.container_id = c.id").
		LeftJoin("volumes pv ON v.parent_id = pv.id").
		LeftJoin("worker_resource_caches wrc ON wrc.id = v.worker_resource_cache_id").
		Where(where).
		RunWith(conn).
		QueryRow()

	creatingVolume, createdVolume, _, _, err := scanVolume(row, conn)
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
	"v.worker_artifact_id",
	`case
	when v.worker_base_resource_type_id is not NULL then 'resource-type'
	when v.worker_resource_cache_id is not NULL then 'resource'
	when v.container_id is not NULL then 'container'
	when v.worker_task_cache_id is not NULL then 'task-cache'
	when v.worker_resource_certs_id is not NULL then 'resource-certs'
	when v.worker_artifact_id is not NULL then 'artifact'
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
	var sqWorkerArtifactID sql.NullInt64
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
		&sqWorkerArtifactID,
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

	var workerArtifactID int
	if sqWorkerArtifactID.Valid {
		workerArtifactID = int(sqWorkerArtifactID.Int64)
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
			workerArtifactID:         workerArtifactID,
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
			workerArtifactID:         workerArtifactID,
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

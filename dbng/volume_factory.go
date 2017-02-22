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

	CreateContainerVolume(int, Worker, CreatingContainer, string) (CreatingVolume, error)
	FindContainerVolume(int, Worker, CreatingContainer, string) (CreatingVolume, CreatedVolume, error)

	FindBaseResourceTypeVolume(int, Worker, *UsedBaseResourceType) (CreatingVolume, CreatedVolume, error)
	CreateBaseResourceTypeVolume(int, Worker, *UsedBaseResourceType) (CreatingVolume, error)

	FindResourceCacheVolume(Worker, *UsedResourceCache) (CreatingVolume, CreatedVolume, error)
	FindResourceCacheInitializedVolume(Worker, *UsedResourceCache) (CreatedVolume, bool, error)
	CreateResourceCacheVolume(Worker, *UsedResourceCache) (CreatingVolume, error)

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
	query, args, err := psql.Select(volumeColumns...).
		From("volumes v").
		LeftJoin("workers w ON v.worker_name = w.name").
		LeftJoin("containers c ON v.container_id = c.id").
		LeftJoin("volumes pv ON v.parent_id = pv.id").
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
	defer rows.Close()

	createdVolumes := []CreatedVolume{}

	for rows.Next() {
		_, createdVolume, _, err := scanVolume(rows, factory.conn)
		if err != nil {
			return nil, err
		}

		createdVolumes = append(createdVolumes, createdVolume)
	}

	return createdVolumes, nil
}

func (factory *volumeFactory) CreateResourceCacheVolume(worker Worker, resourceCache *UsedResourceCache) (CreatingVolume, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	volume, err := factory.createVolume(
		tx,
		0,
		worker,
		map[string]interface{}{"resource_cache_id": resourceCache.ID},
		VolumeTypeResource,
	)

	volume.resourceCacheID = resourceCache.ID
	return volume, nil
}

func (factory *volumeFactory) CreateBaseResourceTypeVolume(teamID int, worker Worker, ubrt *UsedBaseResourceType) (CreatingVolume, error) {
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
			"base_resource_type_id": ubrt.ID,
			"initialized":           true,
		},
		VolumeTypeResourceType,
	)
	if err != nil {
		return nil, err
	}

	volume.baseResourceTypeID = ubrt.ID
	return volume, nil
}

func (factory *volumeFactory) CreateContainerVolume(teamID int, worker Worker, container CreatingContainer, mountPath string) (CreatingVolume, error) {
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
	volume.containerHandle = container.Handle()
	volume.teamID = teamID
	return volume, nil
}

func (factory *volumeFactory) FindVolumesForContainer(container CreatedContainer) ([]CreatedVolume, error) {
	query, args, err := psql.Select(volumeColumns...).
		From("volumes v").
		LeftJoin("workers w ON v.worker_name = w.name").
		LeftJoin("containers c ON v.container_id = c.id").
		LeftJoin("volumes pv ON v.parent_id = pv.id").
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
		_, createdVolume, _, err := scanVolume(rows, factory.conn)
		if err != nil {
			return nil, err
		}

		createdVolumes = append(createdVolumes, createdVolume)
	}

	return createdVolumes, nil
}

func (factory *volumeFactory) FindContainerVolume(teamID int, worker Worker, container CreatingContainer, mountPath string) (CreatingVolume, CreatedVolume, error) {
	return factory.findVolume(teamID, worker, map[string]interface{}{
		"v.container_id": container.ID(),
		"v.path":         mountPath,
	})
}

func (factory *volumeFactory) FindBaseResourceTypeVolume(teamID int, worker Worker, ubrt *UsedBaseResourceType) (CreatingVolume, CreatedVolume, error) {
	return factory.findVolume(teamID, worker, map[string]interface{}{
		"v.base_resource_type_id": ubrt.ID,
	})
}

func (factory *volumeFactory) FindResourceCacheVolume(worker Worker, resourceCache *UsedResourceCache) (CreatingVolume, CreatedVolume, error) {
	return factory.findVolume(0, worker, map[string]interface{}{
		"v.resource_cache_id": resourceCache.ID,
	})
}

func (factory *volumeFactory) FindResourceCacheInitializedVolume(worker Worker, resourceCache *UsedResourceCache) (CreatedVolume, bool, error) {
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
	query, args, err := psql.Select(volumeColumns...).
		From("volumes v").
		LeftJoin("workers w ON v.worker_name = w.name").
		LeftJoin("containers c ON v.container_id = c.id").
		LeftJoin("volumes pv ON v.parent_id = pv.id").
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
		_, createdVolume, destroyingVolume, err := scanVolume(rows, factory.conn)
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
func (factory *volumeFactory) createVolume(
	tx Tx,
	teamID int,
	worker Worker,
	columns map[string]interface{},
	volumeType VolumeType,
) (*creatingVolume, error) {
	var volumeID int
	handle, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	columnNames := []string{"worker_name", "handle"}
	columnValues := []interface{}{worker.Name(), handle.String()}
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
		teamID: teamID,

		conn: factory.conn,
	}, nil
}

func (factory *volumeFactory) findVolume(teamID int, worker Worker, columns map[string]interface{}) (CreatingVolume, CreatedVolume, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, nil, err
	}

	defer tx.Rollback()

	whereClause := sq.Eq{}
	if teamID != 0 {
		whereClause["v.team_id"] = teamID
	}
	if worker != nil {
		whereClause["v.worker_name"] = worker.Name()
	}

	for name, value := range columns {
		whereClause[name] = value
	}

	row := psql.Select(volumeColumns...).
		From("volumes v").
		LeftJoin("workers w ON v.worker_name = w.name").
		LeftJoin("containers c ON v.container_id = c.id").
		LeftJoin("volumes pv ON v.parent_id = pv.id").
		Where(whereClause).
		RunWith(tx).
		QueryRow()
	creatingVolume, createdVolume, _, err := scanVolume(row, factory.conn)
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

	return creatingVolume, createdVolume, nil
}

var volumeColumns = []string{
	"v.id",
	"v.handle",
	"v.state",
	"w.name",
	"w.addr",
	"w.baggageclaim_url",
	"v.path",
	"c.handle",
	"pv.handle",
	"v.team_id",
	"v.resource_cache_id",
	"v.base_resource_type_id",
	`case when v.container_id is not NULL then 'container'
	  when v.resource_cache_id is not NULL then 'resource'
		when v.base_resource_type_id is not NULL then 'resource-type'
		else 'unknown'
	end`,
}

func scanVolume(row sq.RowScanner, conn Conn) (CreatingVolume, CreatedVolume, DestroyingVolume, error) {
	var id int
	var handle string
	var state string
	var workerName string
	var workerAddress string
	var sqWorkerBaggageclaimURL sql.NullString
	var sqPath sql.NullString
	var sqContainerHandle sql.NullString
	var sqParentHandle sql.NullString
	var sqTeamID sql.NullInt64
	var sqResourceCacheID sql.NullInt64
	var sqBaseResourceTypeID sql.NullInt64

	var volumeType VolumeType

	err := row.Scan(
		&id,
		&handle,
		&state,
		&workerName,
		&workerAddress,
		&sqWorkerBaggageclaimURL,
		&sqPath,
		&sqContainerHandle,
		&sqParentHandle,
		&sqTeamID,
		&sqResourceCacheID,
		&sqBaseResourceTypeID,
		&volumeType,
	)
	if err != nil {
		return nil, nil, nil, err
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

	var workerBaggageclaimURL string
	if sqWorkerBaggageclaimURL.Valid {
		workerBaggageclaimURL = sqWorkerBaggageclaimURL.String
	}

	var teamID int
	if sqTeamID.Valid {
		teamID = int(sqTeamID.Int64)
	}

	var resourceCacheID int
	if sqResourceCacheID.Valid {
		resourceCacheID = int(sqResourceCacheID.Int64)
	}

	var baseResourceTypeID int
	if sqBaseResourceTypeID.Valid {
		baseResourceTypeID = int(sqBaseResourceTypeID.Int64)
	}

	switch state {
	case VolumeStateCreated:
		return nil, &createdVolume{
			id:     id,
			handle: handle,
			typ:    volumeType,
			path:   path,
			teamID: teamID,
			worker: &worker{
				name:            workerName,
				gardenAddr:      &workerAddress,
				baggageclaimURL: &workerBaggageclaimURL,
			},
			containerHandle:    containerHandle,
			parentHandle:       parentHandle,
			resourceCacheID:    resourceCacheID,
			baseResourceTypeID: baseResourceTypeID,
			conn:               conn,
		}, nil, nil
	case VolumeStateCreating:
		return &creatingVolume{
			id:     id,
			handle: handle,
			typ:    volumeType,
			path:   path,
			teamID: teamID,
			worker: &worker{
				name:            workerName,
				gardenAddr:      &workerAddress,
				baggageclaimURL: &workerBaggageclaimURL,
			},
			containerHandle:    containerHandle,
			parentHandle:       parentHandle,
			resourceCacheID:    resourceCacheID,
			baseResourceTypeID: baseResourceTypeID,
			conn:               conn,
		}, nil, nil, nil
	case VolumeStateDestroying:
		return nil, nil, &destroyingVolume{
			id:     id,
			handle: handle,
			worker: &worker{
				name:            workerName,
				gardenAddr:      &workerAddress,
				baggageclaimURL: &workerBaggageclaimURL,
			},
			conn: conn,
		}, nil
	}

	return nil, nil, nil, nil
}

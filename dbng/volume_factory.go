package dbng

import (
	"errors"

	sq "github.com/Masterminds/squirrel"
)

type VolumeFactory struct {
	conn Conn
}

func NewVolumeFactory(conn Conn) *VolumeFactory {
	return &VolumeFactory{
		conn: conn,
	}
}

func (factory *VolumeFactory) CreateResourceCacheVolume(worker *Worker, resourceCache *UsedResourceCache) (*CreatingVolume, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()
	return factory.createVolume(tx, worker, "resource_cache_id", resourceCache.ID)
}

func (factory *VolumeFactory) CreateBaseResourceTypeVolume(worker *Worker, ubrt *UsedBaseResourceType) (*CreatingVolume, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	return factory.createVolume(tx, worker, "base_resource_type_id", ubrt.ID)
}

func (factory *VolumeFactory) CreateContainerVolume(worker *Worker, container *CreatingContainer) (*CreatingVolume, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	return factory.createVolume(tx, worker, "container_id", container.ID)
}

func (factory *VolumeFactory) GetOrphanedVolumes() ([]*InitializedVolume, []*DestroyingVolume, error) {
	query, args, err := psql.Select("v.id, v.handle, v.state, w.name, w.addr").
		From("volumes v").
		LeftJoin("workers w ON v.worker_name = w.name").
		Where(sq.Eq{
			"v.resource_cache_id":     nil,
			"v.base_resource_type_id": nil,
			"v.container_id":          nil,
		}).
		ToSql()
	if err != nil {
		return nil, nil, err
	}

	rows, err := factory.conn.Query(query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	initializedVolumes := []*InitializedVolume{}
	destroyingVolumes := []*DestroyingVolume{}

	for rows.Next() {
		var id int
		var handle string
		var state string
		var workerName string
		var workerAddress string

		err = rows.Scan(&id, &handle, &state, &workerName, &workerAddress)
		if err != nil {
			return nil, nil, err
		}
		switch state {
		case VolumeStateInitialized:
			initializedVolumes = append(initializedVolumes, &InitializedVolume{
				ID:     id,
				Handle: handle,
				Worker: &Worker{
					Name:       workerName,
					GardenAddr: workerAddress,
				},
			})
		case VolumeStateDestroying:
			destroyingVolumes = append(destroyingVolumes, &DestroyingVolume{
				ID:     id,
				Handle: handle,
				Worker: &Worker{
					Name:       workerName,
					GardenAddr: workerAddress,
				},
			})
		}
	}

	return initializedVolumes, destroyingVolumes, nil
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
func (factory *VolumeFactory) createVolume(tx Tx, worker *Worker, parentColumnName string, parentColumnValue int) (*CreatingVolume, error) {

	var volumeID int
	err := psql.Insert("volumes").
		Columns("worker_name", parentColumnName).
		Values(worker.Name, parentColumnValue).
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

	return &CreatingVolume{
		Worker: worker,

		ID: volumeID,
	}, nil
}

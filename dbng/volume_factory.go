package dbng

import "errors"

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

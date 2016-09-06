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

func (factory *VolumeFactory) CreateVolume() (*CreatingVolume, error) {
	return &CreatingVolume{
		conn: conn,
	}
}

// 'get' looks like:
//
// 1. lookup cache volume
//	 * if found, goto 4.
// 2. create cache volume
// 3. initialize cache volume
// 4. use cache volume
//   * if false returned, goto 2.

// 1. open tx
// 2. lookup cache id
//   * if not found, create.
//     * if fails (unique violation; concurrent create), goto 1.
// 3. insert into volumes in 'initializing' state
//   * if fails (fkey violation; preexisting cache id was removed), goto 1.
// 4. commit tx
func (factory *VolumeFactory) CreateCacheVolume(cache Cache) (*CreatingVolume, error) {
	return nil, nil
}

var ErrWorkerResourceTypeNotFound = errors.New("worker resource type no longer exists (stale?)")

// 1. open tx
// 2. lookup worker resource type id
//   * if not found, fail; worker must have new version or no longer supports type
// 3. insert into volumes in 'initializing' state
//   * if fails (fkey violation; worker type gone), fail for same reason as 2.
// 4. commit tx
func (factory *VolumeFactory) CreateWorkerResourceTypeVolume(wrt WorkerResourceType) (*CreatingVolume, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	wrtID, found, err := wrt.Lookup(tx)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, ErrWorkerResourceTypeNotFound
	}

	// TODO: worker_name relation is redundant

	var volumeID int
	err = psql.Insert("volumes").
		Columns("worker_name", "worker_resource_type_id").
		Values(wrt.WorkerName, wrtID).
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
		ID: volumeID,
	}, nil
}

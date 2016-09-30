package dbng

import (
	sq "github.com/Masterminds/squirrel"
	uuid "github.com/nu7hatch/gouuid"
)

type VolumeState string

const (
	VolumeStateCreating   = "creating"
	VolumeStateCreated    = "created"
	VolumeStateDestroying = "destroying"
)

// TODO: do not permit nullifying cache_id while creating or created

//go:generate counterfeiter . CreatingVolume

type CreatingVolume interface {
	Handle() string
	Created() (CreatedVolume, error)
}

type creatingVolume struct {
	id     int
	worker *Worker
	handle string
	conn   Conn
}

func (volume *creatingVolume) Handle() string { return volume.handle }

func (volume *creatingVolume) Created() (CreatedVolume, error) {
	tx, err := volume.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	transitioned, err := stateTransition(
		volume.id,
		tx,
		VolumeStateCreating,
		VolumeStateCreated,
		map[string]interface{}{},
	)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	if !transitioned {
		panic("TESTME")
		return nil, nil
	}

	return &createdVolume{
		id:     volume.id,
		worker: volume.worker,
		handle: volume.handle,
		conn:   volume.conn,
	}, nil
}

//go:generate counterfeiter . CreatedVolume

type CreatedVolume interface {
	Handle() string
	CreateChildForContainer(*CreatingContainer) (CreatingVolume, error)
	Destroying() (DestroyingVolume, error)
}

type createdVolume struct {
	id     int
	worker *Worker
	handle string
	conn   Conn
}

func (volume *createdVolume) Handle() string { return volume.handle }

// TODO: do following two methods instead of CreateXVolume? kind of neat since
// it removes window of time where cache_id/worker_resource_type_id may be
// removed while creating/created/initializing, guaranteeing the cache can be
// created within the Initialized call, and *may* remove Tx argument from all
// methods

// func (volume *InitializingVolume) InitializedWorkerResourceType(
// 	wrt WorkerResourceType,
// 	cacheWarmingContainer *CreatingContainer,
// ) (*InitializedVolume, *CreatingVolume, error) {
// 	return nil, nil, nil
// }

// 1. open tx
// 2. set volume state to 'initialized' if 'initializing' or 'initialized'
//    * if fails, return false; must be 'deleting'
// 3. insert into volumes with parent id and parent state
//    * if fails, return false; transitioned to as it was previously 'initialized'
// 4. commit tx
// func (volume *InitializingVolume) InitializedCache(
// 	cache Cache,
// 	cacheUsingContainer *CreatingContainer,
// ) (*InitializedVolume, *CreatingVolume, error) {
// 	var workerName string

// 	// TODO: swap-out container_id with cache_id

// 	err := psql.Update("volumes").
// 		Set("state", VolumeStateInitialized).
// 		Set("container_id", "").
// 		Where(sq.Eq{
// 			"id": volume.ID,

// 			// may have been initialized concurrently
// 			"state": []string{
// 				VolumeStateInitializing,
// 				VolumeStateInitialized,
// 			},
// 		}).
// 		Suffix("RETURNING worker_name").
// 		RunWith(runner).
// 		QueryRow().
// 		Scan(&workerName)
// 	if err != nil {
// 		if err == sql.ErrNoRows {
// 			// TODO: possibly set to 'deleting'; return explicit error
// 			panic("TESTME")
// 		}
// 		return nil, nil, err
// 	}

// 	var creatingID int
// 	err = psql.Insert("volumes").
// 		Columns("worker_name", "parent_id", "parent_state", "container_id").
// 		Values(workerName, volume.ID, VolumeStateInitialized, cacheUsingContainer.ID).
// 		Suffix("RETURNING id").
// 		RunWith(tx).
// 		QueryRow().
// 		Scan(&creatingID)
// 	if err != nil {
// 		// TODO: possible fkey error if parent set to 'deleting'; return explicit error
// 		return nil, nil, err
// 	}

// 	return &InitializedVolume{
// 			InitializingVolume: *volume,
// 		}, &CreatingVolume{
// 			ID: creatingID,
// 		}, nil
// }

func (volume *createdVolume) CreateChildForContainer(container *CreatingContainer) (CreatingVolume, error) {
	tx, err := volume.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	handle, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	var volumeID int
	err = psql.Insert("volumes").
		Columns("worker_name", "parent_id", "parent_state", "handle").
		Values(volume.worker.Name, volume.id, VolumeStateCreated, handle).
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
		id:     volume.id,
		worker: volume.worker,
		handle: volume.handle,
		conn:   volume.conn,
	}, nil
}

func (volume *createdVolume) Destroying() (DestroyingVolume, error) {
	tx, err := volume.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	transitioned, err := stateTransition(volume.id, tx, VolumeStateCreated, VolumeStateDestroying, map[string]interface{}{})
	if err != nil {
		// TODO: return explicit error for failed transition due to volumes using it
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	if !transitioned {
		panic("TESTME")
		return nil, nil
	}

	return &destroyingVolume{
		id:     volume.id,
		worker: volume.worker,
		handle: volume.handle,
		conn:   volume.conn,
	}, nil
}

type DestroyingVolume interface {
	Handle() string
	Destroy() (bool, error)
	Worker() *Worker
}

type destroyingVolume struct {
	id     int
	worker *Worker
	handle string
	conn   Conn
}

func (volume *destroyingVolume) Handle() string  { return volume.handle }
func (volume *destroyingVolume) Worker() *Worker { return volume.worker }

func (volume *destroyingVolume) Destroy() (bool, error) {
	tx, err := volume.conn.Begin()
	if err != nil {
		return false, err
	}

	defer tx.Rollback()

	rows, err := psql.Delete("volumes").
		Where(sq.Eq{
			"id":    volume.id,
			"state": VolumeStateDestroying,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return false, err
	}

	err = tx.Commit()
	if err != nil {
		return false, err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return false, err
	}

	if affected == 0 {
		panic("TESTME")
		return false, nil
	}

	return true, nil
}

func stateTransition(volumeID int, tx Tx, from, to VolumeState, setMap map[string]interface{}) (bool, error) {
	rows, err := psql.Update("volumes").
		Set("state", string(to)).
		SetMap(setMap).
		Where(sq.Eq{
			"id":    volumeID,
			"state": string(from),
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return false, err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return false, err
	}

	if affected == 0 {
		panic("TESTME")
		return false, nil
	}

	return true, nil
}

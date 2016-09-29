package dbng

import sq "github.com/Masterminds/squirrel"

type VolumeState string

const (
	VolumeStateCreating   = "creating"
	VolumeStateCreated    = "created"
	VolumeStateDestroying = "destroying"
)

// TODO: do not permit nullifying cache_id while creating or created

type CreatingVolume struct {
	ID     int
	Worker *Worker
	conn   Conn
}

func (volume *CreatingVolume) Created(handle string) (*CreatedVolume, error) {
	tx, err := volume.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	transitioned, err := stateTransition(
		volume.ID,
		tx,
		VolumeStateCreating,
		VolumeStateCreated,
		map[string]interface{}{"handle": handle},
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

	return &CreatedVolume{
		ID:     volume.ID,
		Worker: volume.Worker,
		Handle: handle,
		conn:   volume.conn,
	}, nil
}

type CreatedVolume struct {
	ID     int
	Worker *Worker
	Handle string
	conn   Conn
}

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

func (volume *CreatedVolume) CreateChildForContainer(container *CreatingContainer) (*CreatingVolume, error) {
	tx, err := volume.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	var volumeID int
	err = psql.Insert("volumes").
		Columns("worker_name", "parent_id", "parent_state").
		Values(volume.Worker.Name, volume.ID, VolumeStateCreated).
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
		ID:     volume.ID,
		Worker: volume.Worker,
		conn:   volume.conn,
	}, nil
}

func (volume *CreatedVolume) Destroying() (*DestroyingVolume, error) {
	tx, err := volume.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	transitioned, err := stateTransition(volume.ID, tx, VolumeStateCreated, VolumeStateDestroying, map[string]interface{}{})
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

	return &DestroyingVolume{
		ID:     volume.ID,
		Worker: volume.Worker,
		Handle: volume.Handle,
		conn:   volume.conn,
	}, nil
}

type DestroyingVolume struct {
	ID     int
	Worker *Worker
	Handle string
	conn   Conn
}

func (volume *DestroyingVolume) Destroy() (bool, error) {
	tx, err := volume.conn.Begin()
	if err != nil {
		return false, err
	}

	defer tx.Rollback()

	rows, err := psql.Delete("volumes").
		Where(sq.Eq{
			"id":    volume.ID,
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

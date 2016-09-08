package dbng

import sq "github.com/Masterminds/squirrel"

type VolumeState string

const (
	VolumeStateCreating     = "creating"
	VolumeStateCreated      = "created"
	VolumeStateInitializing = "initializing"
	VolumeStateInitialized  = "initialized"
	VolumeStateDestroying   = "destroying"
)

// TODO: do not permit nullifying cache_id while creating or created

type CreatingVolume struct {
	ID     int
	Worker *Worker
}

func (volume *CreatingVolume) Created(tx Tx, handle string) (*CreatedVolume, error) {
	rows, err := psql.Update("volumes").
		Set("state", VolumeStateCreated).
		Set("handle", handle).
		Where(sq.Eq{
			"id":    volume.ID,
			"state": VolumeStateCreating,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return nil, err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return nil, err
	}

	if affected == 0 {
		panic("TESTME")
		return nil, nil
	}

	return &CreatedVolume{
		ID:     volume.ID,
		Worker: volume.Worker,
		Handle: handle,
	}, nil
}

type CreatedVolume struct {
	ID     int
	Worker *Worker
	Handle string
}

func (volume *CreatedVolume) Initializing(tx Tx, container *CreatingContainer) (*InitializingVolume, error) {
	transitioned, err := stateTransition(volume.ID, tx, VolumeStateCreated, VolumeStateInitializing)
	if err != nil {
		return nil, err
	}

	if !transitioned {
		panic("TESTME")
		return nil, nil
	}

	return &InitializingVolume{
		ID:     volume.ID,
		Worker: volume.Worker,
		Handle: volume.Handle,
	}, nil
}

type InitializingVolume struct {
	ID     int
	Worker *Worker
	Handle string
}

func (volume *InitializingVolume) Initialized(tx Tx) (*InitializedVolume, error) {
	transitioned, err := stateTransition(volume.ID, tx, VolumeStateInitializing, VolumeStateInitialized)
	if err != nil {
		return nil, err
	}

	if !transitioned {
		panic("TESTME")
		return nil, nil
	}

	return &InitializedVolume{
		ID:     volume.ID,
		Worker: volume.Worker,
		Handle: volume.Handle,
	}, nil
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

type InitializedVolume struct {
	ID     int
	Worker *Worker
	Handle string
}

func (volume *InitializedVolume) CreateChildForContainer(tx Tx, container *CreatingContainer) (*CreatingVolume, error) {
	var volumeID int
	err := psql.Insert("volumes").
		Columns("worker_name", "parent_id", "parent_state").
		Values(volume.Worker.Name, volume.ID, VolumeStateInitialized).
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
	}, nil
}

func (volume *InitializedVolume) Destroying(tx Tx) (*DestroyingVolume, error) {
	transitioned, err := stateTransition(volume.ID, tx, VolumeStateInitialized, VolumeStateDestroying)
	if err != nil {
		// TODO: return explicit error for failed transition due to volumes using it
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
	}, nil
}

type DestroyingVolume struct {
	ID     int
	Worker *Worker
	Handle string
}

func (volume *DestroyingVolume) Destroy(tx Tx) (bool, error) {
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

func stateTransition(volumeID int, tx Tx, from, to VolumeState) (bool, error) {
	rows, err := psql.Update("volumes").
		Set("state", string(to)).
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

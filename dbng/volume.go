package dbng

import (
	"database/sql"
	"encoding/json"
	"errors"

	sq "github.com/Masterminds/squirrel"
	uuid "github.com/nu7hatch/gouuid"

	"github.com/concourse/atc"
)

var (
	ErrVolumeMarkCreatedFailed     = errors.New("could-not-mark-volume-as-created")
	ErrVolumeMarkDestroyingFailed  = errors.New("could-not-mark-volume-as-destroying")
	ErrVolumeStateTransitionFailed = errors.New("could-not-transition-volume-state")
	ErrVolumeMissing               = errors.New("volume-no-longer-in-db")
	ErrInvalidResourceCache        = errors.New("invalid-resource-cache")
)

type VolumeState string

const (
	VolumeStateCreating   = "creating"
	VolumeStateCreated    = "created"
	VolumeStateDestroying = "destroying"
)

type VolumeType string

const (
	VolumeTypeContainer    = "container"
	VolumeTypeResource     = "resource"
	VolumeTypeResourceType = "resource-type"
	VolumeTypeUknown       = "unknown" // for migration to life
)

// TODO: do not permit nullifying cache_id while creating or created

//go:generate counterfeiter . CreatingVolume

type CreatingVolume interface {
	Handle() string
	ID() int
	Created() (CreatedVolume, error)
}

type creatingVolume struct {
	id                 int
	worker             *Worker
	handle             string
	path               string
	teamID             int
	typ                VolumeType
	containerHandle    string
	parentHandle       string
	resourceCacheID    int
	baseResourceTypeID int
	conn               Conn
}

func (volume *creatingVolume) ID() int { return volume.id }

func (volume *creatingVolume) Handle() string { return volume.handle }

func (volume *creatingVolume) Created() (CreatedVolume, error) {
	tx, err := volume.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	err = stateTransition(
		volume.id,
		tx,
		VolumeStateCreating,
		VolumeStateCreated,
	)
	if err != nil {
		if err == ErrVolumeStateTransitionFailed {
			return nil, ErrVolumeMarkCreatedFailed
		}
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &createdVolume{
		id:                 volume.id,
		worker:             volume.worker,
		typ:                volume.typ,
		handle:             volume.handle,
		path:               volume.path,
		teamID:             volume.teamID,
		conn:               volume.conn,
		containerHandle:    volume.containerHandle,
		parentHandle:       volume.parentHandle,
		resourceCacheID:    volume.resourceCacheID,
		baseResourceTypeID: volume.baseResourceTypeID,
	}, nil
}

//go:generate counterfeiter . CreatedVolume

type CreatedVolume interface {
	Handle() string
	Path() string
	Type() VolumeType
	CreateChildForContainer(CreatingContainer, string) (CreatingVolume, error)
	Destroying() (DestroyingVolume, error)
	Worker() *Worker
	SizeInBytes() int64
	Initialize() error
	IsInitialized() (bool, error)
	ContainerHandle() string
	ParentHandle() string
	ResourceType() (*VolumeResourceType, error)
	BaseResourceType() (*WorkerBaseResourceType, error)
}

type createdVolume struct {
	id                 int
	worker             *Worker
	handle             string
	path               string
	teamID             int
	typ                VolumeType
	bytes              int64
	containerHandle    string
	parentHandle       string
	resourceCacheID    int
	baseResourceTypeID int
	conn               Conn
}

type VolumeResourceType struct {
	BaseResourceType *WorkerBaseResourceType
	ResourceType     *VolumeResourceType
	Version          atc.Version
}

func (volume *createdVolume) Handle() string          { return volume.handle }
func (volume *createdVolume) Path() string            { return volume.path }
func (volume *createdVolume) Worker() *Worker         { return volume.worker }
func (volume *createdVolume) SizeInBytes() int64      { return volume.bytes }
func (volume *createdVolume) Type() VolumeType        { return volume.typ }
func (volume *createdVolume) ContainerHandle() string { return volume.containerHandle }
func (volume *createdVolume) ParentHandle() string    { return volume.parentHandle }

func (volume *createdVolume) ResourceType() (*VolumeResourceType, error) {
	if volume.resourceCacheID == 0 {
		return nil, nil
	}

	tx, err := volume.conn.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	return volume.findVolumeResourceTypeByCacheID(tx, volume.resourceCacheID)
}

func (volume *createdVolume) BaseResourceType() (*WorkerBaseResourceType, error) {
	if volume.baseResourceTypeID == 0 {
		return nil, nil
	}

	tx, err := volume.conn.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	return volume.findBaseResourceTypeByID(tx, volume.baseResourceTypeID)
}

func (volume *createdVolume) findVolumeResourceTypeByCacheID(tx Tx, resourceCacheID int) (*VolumeResourceType, error) {
	var versionString []byte
	var sqBaseResourceTypeID sql.NullInt64
	var sqResourceCacheID sql.NullInt64

	err := psql.Select("rc.version, rcfg.base_resource_type_id, rcfg.resource_cache_id").
		From("resource_caches rc").
		LeftJoin("resource_configs rcfg ON rcfg.id = rc.resource_config_id").
		Where(sq.Eq{
			"rc.id": resourceCacheID,
		}).
		RunWith(tx).
		QueryRow().
		Scan(&versionString, &sqBaseResourceTypeID, &sqResourceCacheID)
	if err != nil {
		return nil, err
	}

	var version atc.Version
	err = json.Unmarshal(versionString, &version)
	if err != nil {
		return nil, err
	}

	if sqBaseResourceTypeID.Valid {
		baseResourceType, err := volume.findBaseResourceTypeByID(tx, int(sqBaseResourceTypeID.Int64))
		if err != nil {
			return nil, err
		}

		return &VolumeResourceType{
			BaseResourceType: baseResourceType,
			Version:          version,
		}, nil
	}

	if sqResourceCacheID.Valid {
		resourceType, err := volume.findVolumeResourceTypeByCacheID(tx, int(sqResourceCacheID.Int64))
		if err != nil {
			return nil, err
		}

		return &VolumeResourceType{
			ResourceType: resourceType,
			Version:      version,
		}, nil
	}

	return nil, ErrInvalidResourceCache
}

func (volume *createdVolume) findBaseResourceTypeByID(tx Tx, baseResourceTypeID int) (*WorkerBaseResourceType, error) {
	var name string
	var version string

	err := psql.Select("brt.name, wbrt.version").
		From("worker_base_resource_types wbrt").
		LeftJoin("base_resource_types brt ON brt.id = wbrt.base_resource_type_id").
		Where(sq.Eq{
			"brt.id":           baseResourceTypeID,
			"wbrt.worker_name": volume.worker.Name,
		}).
		RunWith(tx).
		QueryRow().
		Scan(&name, &version)
	if err != nil {
		return nil, err
	}

	return &WorkerBaseResourceType{
		Name:    name,
		Version: version,
	}, nil
}

// TODO: do following two methods instead of CreateXVolume? kind of neat since
// it removes window of time where cache_id/worker_resource_type_id may be
// removed while creating/created/initializing, guaranteeing the cache can be
// created within the Initialized call, and *may* remove Tx argument from all
// methods

// func (volume *InitializingVolume) InitializedWorkerResourceType(
// 	wrt WorkerResourceType,
// 	cacheWarmingContainer CreatingContainer,
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
// 	cacheUsingContainer CreatingContainer,
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

func (volume *createdVolume) Initialize() error {
	tx, err := volume.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := psql.Update("volumes").
		Set("initialized", sq.Expr("true")).
		Where(sq.Eq{
			"id": volume.id,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrVolumeMissing
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (volume *createdVolume) IsInitialized() (bool, error) {
	tx, err := volume.conn.Begin()
	if err != nil {
		return false, err
	}

	defer tx.Rollback()

	var isInitialized bool
	err = psql.Select("initialized").
		From("volumes").
		Where(sq.Eq{
			"id": volume.id,
		}).
		RunWith(tx).
		QueryRow().Scan(&isInitialized)
	if err != nil {
		return false, err
	}

	return isInitialized, nil
}

func (volume *createdVolume) CreateChildForContainer(container CreatingContainer, mountPath string) (CreatingVolume, error) {
	tx, err := volume.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	var parentIsInitialized bool
	err = psql.Select("initialized").
		From("volumes").
		Where(sq.Eq{
			"id": volume.id,
		}).
		RunWith(tx).
		QueryRow().Scan(&parentIsInitialized)
	if err != nil {
		return nil, err
	}

	handle, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	columnNames := []string{
		"worker_name",
		"parent_id",
		"parent_state",
		"handle",
		"container_id",
		"initialized",
		"path",
	}
	columnValues := []interface{}{
		volume.worker.Name,
		volume.id,
		VolumeStateCreated,
		handle.String(),
		container.ID(),
		parentIsInitialized,
		mountPath,
	}

	if volume.teamID != 0 {
		columnNames = append(columnNames, "team_id")
		columnValues = append(columnValues, volume.teamID)
	}

	var volumeID int
	err = psql.Insert("volumes").
		Columns(columnNames...).
		Values(columnValues...).
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
		id:              volumeID,
		worker:          volume.worker,
		handle:          handle.String(),
		path:            mountPath,
		teamID:          volume.teamID,
		typ:             VolumeTypeContainer,
		containerHandle: container.Handle(),
		parentHandle:    volume.Handle(),
		conn:            volume.conn,
	}, nil
}

func (volume *createdVolume) Destroying() (DestroyingVolume, error) {
	tx, err := volume.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	err = stateTransition(
		volume.id,
		tx,
		VolumeStateCreated,
		VolumeStateDestroying,
	)
	if err != nil {
		if err == ErrVolumeStateTransitionFailed {
			return nil, ErrVolumeMarkDestroyingFailed
		}
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
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
		return false, nil
	}

	return true, nil
}

func stateTransition(volumeID int, tx Tx, from, to VolumeState) error {
	rows, err := psql.Update("volumes").
		Set("state", string(to)).
		Where(sq.Eq{
			"id":    volumeID,
			"state": string(from),
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return ErrVolumeStateTransitionFailed
	}

	return nil
}

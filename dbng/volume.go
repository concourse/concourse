package dbng

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	uuid "github.com/nu7hatch/gouuid"

	"github.com/concourse/atc"
)

var (
	ErrVolumeMarkDestroyingFailed  = errors.New("could-not-mark-volume-as-destroying")
	ErrVolumeStateTransitionFailed = errors.New("could-not-transition-volume-state")
	ErrVolumeMissing               = errors.New("volume-no-longer-in-db")
	ErrInvalidResourceCache        = errors.New("invalid-resource-cache")
)

type ErrVolumeMarkCreatedFailed struct {
	Handle string
}

func (e ErrVolumeMarkCreatedFailed) Error() string {
	return fmt.Sprintf("failed to mark volume as created %s", e.Handle)
}

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

//go:generate counterfeiter . CreatingVolume

type CreatingVolume interface {
	Handle() string
	ID() int
	Created() (CreatedVolume, error)
}

type creatingVolume struct {
	id                 int
	worker             Worker
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
			return nil, ErrVolumeMarkCreatedFailed{Handle: volume.handle}
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
	Worker() Worker
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
	worker             Worker
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
func (volume *createdVolume) Worker() Worker          { return volume.worker }
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
			"wbrt.worker_name": volume.worker.Name(),
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
		volume.worker.Name(),
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
	Worker() Worker
}

type destroyingVolume struct {
	id     int
	worker Worker
	handle string
	conn   Conn
}

func (volume *destroyingVolume) Handle() string { return volume.handle }
func (volume *destroyingVolume) Worker() Worker { return volume.worker }

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
		Where(sq.And{
			sq.Eq{"id": volumeID},
			sq.Or{
				sq.Eq{"state": string(from)},
				sq.Eq{"state": string(to)},
			},
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

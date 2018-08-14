package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"
	uuid "github.com/nu7hatch/gouuid"

	"github.com/concourse/atc"
)

var (
	ErrVolumeCannotBeDestroyedWithChildrenPresent = errors.New("volume cannot be destroyed as children are present")
	ErrVolumeStateTransitionFailed                = errors.New("could not transition volume state")
	ErrVolumeMissing                              = errors.New("volume no longer in db")
	ErrInvalidResourceCache                       = errors.New("invalid resource cache")
)

type ErrVolumeMarkStateFailed struct {
	State VolumeState
}

func (e ErrVolumeMarkStateFailed) Error() string {
	return fmt.Sprintf("could not mark volume as %s", e.State)
}

type ErrVolumeMarkCreatedFailed struct {
	Handle string
}

func (e ErrVolumeMarkCreatedFailed) Error() string {
	return fmt.Sprintf("failed to mark volume as created %s", e.Handle)
}

type VolumeState string

const (
	VolumeStateCreating   VolumeState = "creating"
	VolumeStateCreated    VolumeState = "created"
	VolumeStateDestroying VolumeState = "destroying"
	VolumeStateFailed     VolumeState = "failed"
)

type VolumeType string

const (
	VolumeTypeContainer     VolumeType = "container"
	VolumeTypeResource      VolumeType = "resource"
	VolumeTypeResourceType  VolumeType = "resource-type"
	VolumeTypeResourceCerts VolumeType = "resource-certs"
	VolumeTypeTaskCache     VolumeType = "task-cache"
	VolumeTypeUknown        VolumeType = "unknown" // for migration to life
)

//go:generate counterfeiter . CreatingVolume

type CreatingVolume interface {
	Handle() string
	ID() int
	Created() (CreatedVolume, error)
	Failed() (FailedVolume, error)
}

type creatingVolume struct {
	id                       int
	workerName               string
	handle                   string
	path                     string
	teamID                   int
	typ                      VolumeType
	containerHandle          string
	parentHandle             string
	resourceCacheID          int
	workerBaseResourceTypeID int
	workerTaskCacheID        int
	workerResourceCertsID    int
	conn                     Conn
}

func (volume *creatingVolume) ID() int { return volume.id }

func (volume *creatingVolume) Handle() string { return volume.handle }

func (volume *creatingVolume) Created() (CreatedVolume, error) {
	err := volumeStateTransition(
		volume.id,
		volume.conn,
		VolumeStateCreating,
		VolumeStateCreated,
	)
	if err != nil {
		if err == ErrVolumeStateTransitionFailed {
			return nil, ErrVolumeMarkCreatedFailed{Handle: volume.handle}
		}
		return nil, err
	}

	return &createdVolume{
		id:                       volume.id,
		workerName:               volume.workerName,
		typ:                      volume.typ,
		handle:                   volume.handle,
		path:                     volume.path,
		teamID:                   volume.teamID,
		conn:                     volume.conn,
		containerHandle:          volume.containerHandle,
		parentHandle:             volume.parentHandle,
		resourceCacheID:          volume.resourceCacheID,
		workerBaseResourceTypeID: volume.workerBaseResourceTypeID,
		workerTaskCacheID:        volume.workerTaskCacheID,
		workerResourceCertsID:    volume.workerResourceCertsID,
	}, nil
}

func (volume *creatingVolume) Failed() (FailedVolume, error) {
	err := volumeStateTransition(
		volume.id,
		volume.conn,
		VolumeStateCreating,
		VolumeStateFailed,
	)
	if err != nil {
		if err == ErrVolumeStateTransitionFailed {
			return nil, ErrVolumeMarkStateFailed{VolumeStateFailed}
		}
		return nil, err
	}

	return &failedVolume{
		id:         volume.id,
		workerName: volume.workerName,
		handle:     volume.handle,
		conn:       volume.conn,
	}, nil
}

//go:generate counterfeiter . CreatedVolume

type CreatedVolume interface {
	Handle() string
	Path() string
	Type() VolumeType
	TeamID() int
	CreateChildForContainer(CreatingContainer, string) (CreatingVolume, error)
	Destroying() (DestroyingVolume, error)
	WorkerName() string
	InitializeResourceCache(UsedResourceCache) error
	InitializeTaskCache(int, string, string) error
	ContainerHandle() string
	ParentHandle() string
	ResourceType() (*VolumeResourceType, error)
	BaseResourceType() (*UsedWorkerBaseResourceType, error)
	TaskIdentifier() (string, string, string, error)
}

type createdVolume struct {
	id                       int
	workerName               string
	handle                   string
	path                     string
	teamID                   int
	typ                      VolumeType
	containerHandle          string
	parentHandle             string
	resourceCacheID          int
	workerBaseResourceTypeID int
	workerTaskCacheID        int
	workerResourceCertsID    int
	conn                     Conn
}

type VolumeResourceType struct {
	WorkerBaseResourceType *UsedWorkerBaseResourceType
	ResourceType           *VolumeResourceType
	Version                atc.Version
}

func (volume *createdVolume) Handle() string          { return volume.handle }
func (volume *createdVolume) Path() string            { return volume.path }
func (volume *createdVolume) WorkerName() string      { return volume.workerName }
func (volume *createdVolume) Type() VolumeType        { return volume.typ }
func (volume *createdVolume) TeamID() int             { return volume.teamID }
func (volume *createdVolume) ContainerHandle() string { return volume.containerHandle }
func (volume *createdVolume) ParentHandle() string    { return volume.parentHandle }

func (volume *createdVolume) ResourceType() (*VolumeResourceType, error) {
	if volume.resourceCacheID == 0 {
		return nil, nil
	}

	return volume.findVolumeResourceTypeByCacheID(volume.resourceCacheID)
}

func (volume *createdVolume) BaseResourceType() (*UsedWorkerBaseResourceType, error) {
	if volume.workerBaseResourceTypeID == 0 {
		return nil, nil
	}

	return volume.findWorkerBaseResourceTypeByID(volume.workerBaseResourceTypeID)
}

func (volume *createdVolume) TaskIdentifier() (string, string, string, error) {
	if volume.workerTaskCacheID == 0 {
		return "", "", "", nil
	}

	var pipelineName string
	var jobName string
	var stepName string

	err := psql.Select("p.name, j.name, wtc.step_name").
		From("worker_task_caches wtc").
		LeftJoin("jobs j ON j.id = wtc.job_id").
		LeftJoin("pipelines p ON p.id = j.pipeline_id").
		Where(sq.Eq{
			"wtc.id": volume.workerTaskCacheID,
		}).
		RunWith(volume.conn).
		QueryRow().
		Scan(&pipelineName, &jobName, &stepName)
	if err != nil {
		return "", "", "", err
	}

	return pipelineName, jobName, stepName, nil
}

func (volume *createdVolume) findVolumeResourceTypeByCacheID(resourceCacheID int) (*VolumeResourceType, error) {
	var versionString []byte
	var sqBaseResourceTypeID sql.NullInt64
	var sqResourceCacheID sql.NullInt64

	err := psql.Select("rc.version, rcfg.base_resource_type_id, rcfg.resource_cache_id").
		From("resource_caches rc").
		LeftJoin("resource_configs rcfg ON rcfg.id = rc.resource_config_id").
		Where(sq.Eq{
			"rc.id": resourceCacheID,
		}).
		RunWith(volume.conn).
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
		workerBaseResourceType, err := volume.findWorkerBaseResourceTypeByBaseResourceTypeID(int(sqBaseResourceTypeID.Int64))
		if err != nil {
			return nil, err
		}

		return &VolumeResourceType{
			WorkerBaseResourceType: workerBaseResourceType,
			Version:                version,
		}, nil
	}

	if sqResourceCacheID.Valid {
		resourceType, err := volume.findVolumeResourceTypeByCacheID(int(sqResourceCacheID.Int64))
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

func (volume *createdVolume) findWorkerBaseResourceTypeByID(workerBaseResourceTypeID int) (*UsedWorkerBaseResourceType, error) {
	var name string
	var version string

	err := psql.Select("brt.name, wbrt.version").
		From("worker_base_resource_types wbrt").
		LeftJoin("base_resource_types brt ON brt.id = wbrt.base_resource_type_id").
		Where(sq.Eq{
			"wbrt.id":          workerBaseResourceTypeID,
			"wbrt.worker_name": volume.workerName,
		}).
		RunWith(volume.conn).
		QueryRow().
		Scan(&name, &version)
	if err != nil {
		return nil, err
	}

	return &UsedWorkerBaseResourceType{
		ID:         workerBaseResourceTypeID,
		Name:       name,
		Version:    version,
		WorkerName: volume.workerName,
	}, nil
}

func (volume *createdVolume) findWorkerBaseResourceTypeByBaseResourceTypeID(baseResourceTypeID int) (*UsedWorkerBaseResourceType, error) {
	var id int
	var name string
	var version string

	err := psql.Select("wbrt.id, brt.name, wbrt.version").
		From("worker_base_resource_types wbrt").
		LeftJoin("base_resource_types brt ON brt.id = wbrt.base_resource_type_id").
		Where(sq.Eq{
			"brt.id":           baseResourceTypeID,
			"wbrt.worker_name": volume.workerName,
		}).
		RunWith(volume.conn).
		QueryRow().
		Scan(&id, &name, &version)
	if err != nil {
		return nil, err
	}

	return &UsedWorkerBaseResourceType{
		ID:         id,
		Name:       name,
		Version:    version,
		WorkerName: volume.workerName,
	}, nil
}

func (volume *createdVolume) InitializeResourceCache(resourceCache UsedResourceCache) error {
	tx, err := volume.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	workerResourceCache, err := WorkerResourceCache{
		WorkerName:    volume.WorkerName(),
		ResourceCache: resourceCache,
	}.FindOrCreate(tx)
	if err != nil {
		return err
	}

	rows, err := psql.Update("volumes").
		Set("worker_resource_cache_id", workerResourceCache.ID).
		Set("team_id", nil).
		Where(sq.Eq{"id": volume.id}).
		RunWith(tx).
		Exec()
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == pqUniqueViolationErrCode {
			// another volume was 'blessed' as the cache volume - leave this one
			// owned by the container so it just expires when the container is GCed
			return nil
		}

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

	volume.resourceCacheID = resourceCache.ID()
	volume.typ = VolumeTypeResource

	return nil
}

func (volume *createdVolume) InitializeTaskCache(jobID int, stepName string, path string) error {
	tx, err := volume.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	usedWorkerTaskCache, err := WorkerTaskCache{
		JobID:      jobID,
		StepName:   stepName,
		WorkerName: volume.WorkerName(),
		Path:       path,
	}.FindOrCreate(tx)
	if err != nil {
		return err
	}

	// release other old volumes for gc
	_, err = psql.Update("volumes").
		Set("worker_task_cache_id", nil).
		Where(sq.Eq{"worker_task_cache_id": usedWorkerTaskCache.ID}).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	rows, err := psql.Update("volumes").
		Set("worker_task_cache_id", usedWorkerTaskCache.ID).
		Where(sq.Eq{"id": volume.id}).
		RunWith(tx).
		Exec()
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == pqUniqueViolationErrCode {
			// another volume was 'blessed' as the cache volume - leave this one
			// owned by the container so it just expires when the container is GCed
			return nil
		}

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

func (volume *createdVolume) CreateChildForContainer(container CreatingContainer, mountPath string) (CreatingVolume, error) {
	tx, err := volume.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

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
		"path",
	}
	columnValues := []interface{}{
		volume.workerName,
		volume.id,
		VolumeStateCreated,
		handle.String(),
		container.ID(),
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
		workerName:      volume.workerName,
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
	err := volumeStateTransition(
		volume.id,
		volume.conn,
		VolumeStateCreated,
		VolumeStateDestroying,
	)
	if err != nil {
		if err == ErrVolumeStateTransitionFailed {
			return nil, ErrVolumeMarkStateFailed{VolumeStateDestroying}

		}

		if pqErr, ok := err.(*pq.Error); ok &&
			pqErr.Code.Name() == pqFKeyViolationErrCode &&
			pqErr.Constraint == "volumes_parent_id_fkey" {
			return nil, ErrVolumeCannotBeDestroyedWithChildrenPresent
		}

		return nil, err
	}

	return &destroyingVolume{
		id:         volume.id,
		workerName: volume.workerName,
		handle:     volume.handle,
		conn:       volume.conn,
	}, nil
}

//go:generate counterfeiter . DestroyingVolume
type DestroyingVolume interface {
	Handle() string
	Destroy() (bool, error)
	WorkerName() string
}

type destroyingVolume struct {
	id         int
	workerName string
	handle     string
	conn       Conn
}

func (volume *destroyingVolume) Handle() string     { return volume.handle }
func (volume *destroyingVolume) WorkerName() string { return volume.workerName }

func (volume *destroyingVolume) Destroy() (bool, error) {
	rows, err := psql.Delete("volumes").
		Where(sq.Eq{
			"id":    volume.id,
			"state": VolumeStateDestroying,
		}).
		RunWith(volume.conn).
		Exec()
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

type FailedVolume interface {
	Handle() string
	Destroy() (bool, error)
	WorkerName() string
}

type failedVolume struct {
	id         int
	workerName string
	handle     string
	conn       Conn
}

func (volume *failedVolume) Handle() string     { return volume.handle }
func (volume *failedVolume) WorkerName() string { return volume.workerName }

func (volume *failedVolume) Destroy() (bool, error) {
	rows, err := psql.Delete("volumes").
		Where(sq.Eq{
			"id":    volume.id,
			"state": VolumeStateFailed,
		}).
		RunWith(volume.conn).
		Exec()
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

func volumeStateTransition(volumeID int, conn Conn, from, to VolumeState) error {
	rows, err := psql.Update("volumes").
		Set("state", string(to)).
		Where(sq.And{
			sq.Eq{"id": volumeID},
			sq.Or{
				sq.Eq{"state": string(from)},
				sq.Eq{"state": string(to)},
			},
		}).
		RunWith(conn).
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

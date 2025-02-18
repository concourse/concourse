package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	uuid "github.com/nu7hatch/gouuid"
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
	VolumeTypeArtifact      VolumeType = "artifact"
	VolumeTypeUknown        VolumeType = "unknown" // for migration to life
)

//counterfeiter:generate . CreatingVolume
type CreatingVolume interface {
	Handle() string
	ID() int
	Created() (CreatedVolume, error)
	Failed() (FailedVolume, error)

	InitializeArtifact() (WorkerArtifact, error)
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
	workerArtifactID         int
	conn                     DbConn
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

func (volume *creatingVolume) InitializeArtifact() (WorkerArtifact, error) {
	return initializeArtifact(volume.conn, volume.id, "", 0)
}

//counterfeiter:generate . CreatedVolume

// TODO-Later Consider separating CORE & Runtime concerns by breaking this abstraction up.
type CreatedVolume interface {
	Handle() string
	Path() string
	Type() VolumeType
	TeamID() int
	WorkerArtifactID() int
	WorkerResourceCacheID() int
	CreateChildForContainer(CreatingContainer, string) (CreatingVolume, error)
	Destroying() (DestroyingVolume, error)
	WorkerName() string

	InitializeResourceCache(ResourceCache) (*UsedWorkerResourceCache, error)
	InitializeStreamedResourceCache(ResourceCache, int) (*UsedWorkerResourceCache, error)
	GetResourceCacheID() int
	InitializeArtifact(name string, buildID int) (WorkerArtifact, error)
	InitializeTaskCache(jobID int, stepName string, path string) error

	ContainerHandle() string
	ParentHandle() string
	ResourceType() (*VolumeResourceType, error)
	BaseResourceType() (*UsedWorkerBaseResourceType, error)
	TaskIdentifier() (int, atc.PipelineRef, string, string, error)
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
	workerResourceCacheID    int
	resourceCacheID          int
	workerBaseResourceTypeID int
	workerTaskCacheID        int
	workerResourceCertsID    int
	workerArtifactID         int
	conn                     DbConn
}

type VolumeResourceType struct {
	WorkerBaseResourceType *UsedWorkerBaseResourceType
	ResourceType           *VolumeResourceType
	Version                atc.Version
}

func (volume *createdVolume) Handle() string             { return volume.handle }
func (volume *createdVolume) Path() string               { return volume.path }
func (volume *createdVolume) WorkerName() string         { return volume.workerName }
func (volume *createdVolume) Type() VolumeType           { return volume.typ }
func (volume *createdVolume) TeamID() int                { return volume.teamID }
func (volume *createdVolume) ContainerHandle() string    { return volume.containerHandle }
func (volume *createdVolume) ParentHandle() string       { return volume.parentHandle }
func (volume *createdVolume) WorkerArtifactID() int      { return volume.workerArtifactID }
func (volume *createdVolume) WorkerResourceCacheID() int { return volume.workerResourceCacheID }

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

func (volume *createdVolume) TaskIdentifier() (int, atc.PipelineRef, string, string, error) {
	if volume.workerTaskCacheID == 0 {
		return 0, atc.PipelineRef{}, "", "", nil
	}

	var pipelineID int
	var pipelineName string
	var pipelineInstanceVars sql.NullString
	var jobName string
	var stepName string

	err := psql.Select("p.id, p.name, p.instance_vars, j.name, tc.step_name").
		From("worker_task_caches wtc").
		LeftJoin("task_caches tc on tc.id = wtc.task_cache_id").
		LeftJoin("jobs j ON j.id = tc.job_id").
		LeftJoin("pipelines p ON p.id = j.pipeline_id").
		Where(sq.Eq{
			"wtc.id": volume.workerTaskCacheID,
		}).
		RunWith(volume.conn).
		QueryRow().
		Scan(&pipelineID, &pipelineName, &pipelineInstanceVars, &jobName, &stepName)
	if err != nil {
		return 0, atc.PipelineRef{}, "", "", err
	}

	pipelineRef := atc.PipelineRef{Name: pipelineName}
	if pipelineInstanceVars.Valid {
		err = json.Unmarshal([]byte(pipelineInstanceVars.String), &pipelineRef.InstanceVars)
		if err != nil {
			return 0, atc.PipelineRef{}, "", "", err
		}
	}

	return pipelineID, pipelineRef, jobName, stepName, nil
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
		workerBaseResourceType, err := volume.findWorkerBaseResourceTypeByBaseResourceTypeID()
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

func (volume *createdVolume) findWorkerBaseResourceTypeByBaseResourceTypeID() (*UsedWorkerBaseResourceType, error) {
	var id int
	var name string
	var version string

	err := psql.Select("wbrt.id, brt.name, wbrt.version").
		From("worker_resource_caches wrc").
		LeftJoin("worker_base_resource_types wbrt ON wbrt.id = wrc.worker_base_resource_type_id").
		LeftJoin("base_resource_types brt ON brt.id = wbrt.base_resource_type_id").
		Where(sq.Eq{
			"wrc.resource_cache_id": volume.resourceCacheID,
			"wrc.worker_name":       volume.workerName,
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

func (volume *createdVolume) InitializeResourceCache(resourceCache ResourceCache) (*UsedWorkerResourceCache, error) {
	tx, err := volume.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	usedWorkerBaseResourceType, found, err := WorkerBaseResourceType{
		Name:       resourceCache.BaseResourceType().Name,
		WorkerName: volume.workerName,
	}.Find(tx)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrWorkerBaseResourceTypeDisappeared
	}

	uwrc, initialized, err := volume.initializeResourceCache(tx, resourceCache, usedWorkerBaseResourceType.ID)
	if err != nil {
		return nil, err
	}
	if !initialized {
		// Another volume became the resource cache - don't commit the transaction
		return nil, nil
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return uwrc, nil
}

func (volume *createdVolume) InitializeStreamedResourceCache(resourceCache ResourceCache, sourceWorkerResourceCacheID int) (*UsedWorkerResourceCache, error) {
	tx, err := volume.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	sourceWorkerResourceCache, found, err := WorkerResourceCache{}.FindByID(tx, sourceWorkerResourceCacheID)
	if err != nil {
		return nil, err
	}
	if !found {
		// resource cache disappeared from source worker. this means the cache
		// was invalidated *after* we started the step that streamed the
		// volume. in this case, we don't want to initialize this volume as a
		// cache, but we also don't want to error the build.
		return nil, nil
	}
	if sourceWorkerResourceCache.WorkerBaseResourceTypeID == 0 {
		// same as not found above. If source worker resource cache's worker
		// base resource type id is 0, that means source worker's base resource
		// type has been invalidated, in this case, we don't want to initialize
		// this volume as a cache, but we also don't want to error the build.
		return nil, nil
	}

	uwrc, initialized, err := volume.initializeResourceCache(tx, resourceCache, sourceWorkerResourceCache.WorkerBaseResourceTypeID)
	if err != nil {
		return nil, err
	}
	if !initialized {
		// Another volume became the resource cache - don't commit the transaction
		return nil, nil
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return uwrc, nil
}

// initializeResourceCache creates a worker resource cache and point current volume's
// worker_resource_cache_id to the cache. When initializing a local generated resource
// cache, then source worker is just the volume's worker.
func (volume *createdVolume) initializeResourceCache(tx Tx, resourceCache ResourceCache, workerBaseResourceTypeID int) (*UsedWorkerResourceCache, bool, error) {
	workerResourceCache, valid, err := WorkerResourceCache{
		WorkerName:    volume.WorkerName(),
		ResourceCache: resourceCache,
	}.FindOrCreate(tx, workerBaseResourceTypeID)
	if err != nil {
		return nil, false, err
	}
	if !valid {
		// there's already a WorkerResourceCache for this resource cache on
		// this worker, but originating from a different sourceWorker, meaning
		// the other streamed volume "won the race", and will become the cached
		// volume
		return nil, false, nil
	}

	rows, err := psql.Update("volumes").
		Set("worker_resource_cache_id", workerResourceCache.ID).
		Set("team_id", nil).
		Where(sq.Eq{"id": volume.id}).
		RunWith(tx).
		Exec()
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == pgerrcode.UniqueViolation {
			// another volume was 'blessed' as the cache volume - leave this one
			// owned by the container so it just expires when the container is GCed
			return nil, false, nil
		}

		return nil, false, err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return nil, false, err
	}

	if affected == 0 {
		return nil, false, ErrVolumeMissing
	}

	volume.workerResourceCacheID = workerResourceCache.ID
	volume.resourceCacheID = resourceCache.ID()
	volume.typ = VolumeTypeResource

	return workerResourceCache, true, nil
}

func (volume *createdVolume) GetResourceCacheID() int {
	return volume.resourceCacheID
}

func (volume *createdVolume) InitializeArtifact(name string, buildID int) (WorkerArtifact, error) {
	return initializeArtifact(volume.conn, volume.id, name, buildID)
}

func initializeArtifact(conn DbConn, volumeID int, name string, buildID int) (WorkerArtifact, error) {
	tx, err := conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	atcWorkerArtifact := atc.WorkerArtifact{
		Name:    name,
		BuildID: buildID,
	}

	workerArtifact, err := saveWorkerArtifact(tx, conn, atcWorkerArtifact)
	if err != nil {
		return nil, err
	}

	rows, err := psql.Update("volumes").
		Set("worker_artifact_id", workerArtifact.ID()).
		Where(sq.Eq{"id": volumeID}).
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
		return nil, ErrVolumeMissing
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return workerArtifact, nil
}

func (volume *createdVolume) InitializeTaskCache(jobID int, stepName string, path string) error {
	tx, err := volume.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	usedTaskCache, err := usedTaskCache{
		jobID:    jobID,
		stepName: stepName,
		path:     path,
	}.findOrCreate(tx)
	if err != nil {
		return err
	}

	usedWorkerTaskCache, err := WorkerTaskCache{
		WorkerName: volume.WorkerName(),
		TaskCache:  usedTaskCache,
	}.findOrCreate(tx)
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
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == pgerrcode.UniqueViolation {
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

		if pgErr, ok := err.(*pgconn.PgError); ok &&
			pgErr.Code == pgerrcode.ForeignKeyViolation &&
			pgErr.ConstraintName == "volumes_parent_id_fkey" {
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

//counterfeiter:generate . DestroyingVolume
type DestroyingVolume interface {
	Handle() string
	Destroy() (bool, error)
	WorkerName() string
}

type destroyingVolume struct {
	id         int
	workerName string
	handle     string
	conn       DbConn
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
	conn       DbConn
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

func volumeStateTransition(volumeID int, conn DbConn, from, to VolumeState) error {
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

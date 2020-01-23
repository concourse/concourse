package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/lib/pq"
	uuid "github.com/nu7hatch/gouuid"
)

var (
	ErrWorkerNotPresent         = errors.New("worker not present in db")
	ErrCannotPruneRunningWorker = errors.New("worker not stalled for pruning")
)

type ContainerOwnerDisappearedError struct {
	owner ContainerOwner
}

func (e ContainerOwnerDisappearedError) Error() string {
	return fmt.Sprintf("container owner %T disappeared", e.owner)
}

type WorkerState string

const (
	WorkerStateRunning  = WorkerState("running")
	WorkerStateStalled  = WorkerState("stalled")
	WorkerStateLanding  = WorkerState("landing")
	WorkerStateLanded   = WorkerState("landed")
	WorkerStateRetiring = WorkerState("retiring")
)

func AllWorkerStates() []WorkerState {
	return []WorkerState{
		WorkerStateRunning,
		WorkerStateStalled,
		WorkerStateLanding,
		WorkerStateLanded,
		WorkerStateRetiring,
	}
}

//go:generate counterfeiter . Worker

type Worker interface {
	Name() string
	Version() *string
	State() WorkerState
	GardenAddr() *string
	BaggageclaimURL() *string
	CertsPath() *string
	ResourceCerts() (*UsedWorkerResourceCerts, bool, error)
	HTTPProxyURL() string
	HTTPSProxyURL() string
	NoProxy() string
	ActiveContainers() int
	ActiveVolumes() int
	ResourceTypes() []atc.WorkerResourceType
	Platform() string
	Tags() []string
	TeamID() int
	TeamName() string
	StartTime() time.Time
	ExpiresAt() time.Time
	Ephemeral() bool

	Reload() (bool, error)

	Land() error
	Retire() error
	Prune() error
	Delete() error

	ActiveTasks() (int, error)
	IncreaseActiveTasks() error
	DecreaseActiveTasks() error

	FindContainer(owner ContainerOwner) (CreatingContainer, CreatedContainer, error)
	CreateContainer(owner ContainerOwner, meta ContainerMetadata) (CreatingContainer, error)
}

type worker struct {
	conn Conn

	name             string
	version          *string
	state            WorkerState
	gardenAddr       *string
	baggageclaimURL  *string
	httpProxyURL     string
	httpsProxyURL    string
	noProxy          string
	activeContainers int
	activeVolumes    int
	activeTasks      int
	resourceTypes    []atc.WorkerResourceType
	platform         string
	tags             []string
	teamID           int
	teamName         string
	startTime        time.Time
	expiresAt        time.Time
	certsPath        *string
	ephemeral        bool
}

func (worker *worker) Name() string             { return worker.name }
func (worker *worker) Version() *string         { return worker.version }
func (worker *worker) State() WorkerState       { return worker.state }
func (worker *worker) GardenAddr() *string      { return worker.gardenAddr }
func (worker *worker) CertsPath() *string       { return worker.certsPath }
func (worker *worker) BaggageclaimURL() *string { return worker.baggageclaimURL }

func (worker *worker) HTTPProxyURL() string                    { return worker.httpProxyURL }
func (worker *worker) HTTPSProxyURL() string                   { return worker.httpsProxyURL }
func (worker *worker) NoProxy() string                         { return worker.noProxy }
func (worker *worker) ActiveContainers() int                   { return worker.activeContainers }
func (worker *worker) ActiveVolumes() int                      { return worker.activeVolumes }
func (worker *worker) ResourceTypes() []atc.WorkerResourceType { return worker.resourceTypes }
func (worker *worker) Platform() string                        { return worker.platform }
func (worker *worker) Tags() []string                          { return worker.tags }
func (worker *worker) TeamID() int                             { return worker.teamID }
func (worker *worker) TeamName() string                        { return worker.teamName }
func (worker *worker) Ephemeral() bool                         { return worker.ephemeral }

func (worker *worker) StartTime() time.Time { return worker.startTime }
func (worker *worker) ExpiresAt() time.Time { return worker.expiresAt }

func (worker *worker) Reload() (bool, error) {
	row := workersQuery.Where(sq.Eq{"w.name": worker.name}).
		RunWith(worker.conn).
		QueryRow()

	err := scanWorker(worker, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (worker *worker) Land() error {
	cSQL, _, err := sq.Case("state").
		When("'landed'::worker_state", "'landed'::worker_state").
		Else("'landing'::worker_state").
		ToSql()
	if err != nil {
		return err
	}

	result, err := psql.Update("workers").
		Set("state", sq.Expr("("+cSQL+")")).
		Where(sq.Eq{"name": worker.name}).
		RunWith(worker.conn).
		Exec()

	if err != nil {
		return err
	}

	count, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if count == 0 {
		return ErrWorkerNotPresent
	}

	return nil
}

func (worker *worker) Retire() error {
	result, err := psql.Update("workers").
		SetMap(map[string]interface{}{
			"state": string(WorkerStateRetiring),
		}).
		Where(sq.Eq{"name": worker.name}).
		RunWith(worker.conn).
		Exec()
	if err != nil {
		return err
	}

	count, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if count == 0 {
		return ErrWorkerNotPresent
	}

	return nil
}

func (worker *worker) Prune() error {
	tx, err := worker.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	rows, err := sq.Delete("workers").
		Where(sq.Eq{
			"name": worker.name,
		}).
		Where(sq.NotEq{
			"state": string(WorkerStateRunning),
		}).
		PlaceholderFormat(sq.Dollar).
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
		//check whether the worker exists in the database at all
		var one int
		err := psql.Select("1").From("workers").Where(sq.Eq{"name": worker.name}).
			RunWith(tx).
			QueryRow().
			Scan(&one)
		if err != nil {
			if err == sql.ErrNoRows {
				return ErrWorkerNotPresent
			}
			return err
		}

		return ErrCannotPruneRunningWorker
	}

	return tx.Commit()
}

func (worker *worker) Delete() error {
	_, err := sq.Delete("workers").
		Where(sq.Eq{
			"name": worker.name,
		}).
		PlaceholderFormat(sq.Dollar).
		RunWith(worker.conn).
		Exec()

	return err
}

func (worker *worker) ResourceCerts() (*UsedWorkerResourceCerts, bool, error) {
	if worker.certsPath != nil {
		wrc := &WorkerResourceCerts{
			WorkerName: worker.name,
			CertsPath:  *worker.certsPath,
		}

		return wrc.Find(worker.conn)
	}

	return nil, false, nil
}

func (worker *worker) FindContainer(owner ContainerOwner) (CreatingContainer, CreatedContainer, error) {
	ownerQuery, found, err := owner.Find(worker.conn)
	if err != nil {
		return nil, nil, err
	}

	if !found {
		return nil, nil, nil
	}

	return worker.findContainer(sq.And{
		sq.Eq{"worker_name": worker.name},
		ownerQuery,
	})
}

func (worker *worker) CreateContainer(owner ContainerOwner, meta ContainerMetadata) (CreatingContainer, error) {
	handle, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	var containerID int
	cols := []interface{}{&containerID}

	metadata := &ContainerMetadata{}
	cols = append(cols, metadata.ScanTargets()...)

	tx, err := worker.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	insMap := meta.SQLMap()
	insMap["worker_name"] = worker.name
	insMap["handle"] = handle.String()

	ownerCols, err := owner.Create(tx, worker.name)
	if err != nil {
		return nil, err
	}

	for k, v := range ownerCols {
		insMap[k] = v
	}

	err = psql.Insert("containers").
		SetMap(insMap).
		Suffix("RETURNING id, " + strings.Join(containerMetadataColumns, ", ")).
		RunWith(tx).
		QueryRow().
		Scan(cols...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == pqFKeyViolationErrCode {
			return nil, ContainerOwnerDisappearedError{owner}
		}

		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return newCreatingContainer(
		containerID,
		handle.String(),
		worker.name,
		*metadata,
		worker.conn,
	), nil
}

func (worker *worker) findContainer(whereClause sq.Sqlizer) (CreatingContainer, CreatedContainer, error) {
	creating, created, destroying, _, err := scanContainer(
		selectContainers().
			Where(whereClause).
			RunWith(worker.conn).
			QueryRow(),
		worker.conn,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	if destroying != nil {
		return nil, nil, nil
	}

	return creating, created, nil
}

func (worker *worker) ActiveTasks() (int, error) {
	err := psql.Select("active_tasks").From("workers").Where(sq.Eq{"name": worker.name}).
		RunWith(worker.conn).
		QueryRow().
		Scan(&worker.activeTasks)
	if err != nil {
		return 0, err
	}
	return worker.activeTasks, nil
}

func (worker *worker) IncreaseActiveTasks() error {
	result, err := psql.Update("workers").
		Set("active_tasks", sq.Expr("active_tasks+1")).
		Where(sq.Eq{"name": worker.name}).
		RunWith(worker.conn).
		Exec()
	if err != nil {
		return err
	}

	count, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if count == 0 {
		return ErrWorkerNotPresent
	}

	return nil
}

func (worker *worker) DecreaseActiveTasks() error {
	result, err := psql.Update("workers").
		Set("active_tasks", sq.Expr("active_tasks-1")).
		Where(sq.Eq{"name": worker.name}).
		RunWith(worker.conn).
		Exec()
	if err != nil {
		return err
	}

	count, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if count == 0 {
		return ErrWorkerNotPresent
	}

	return nil
}

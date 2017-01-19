package dbng

import (
	"database/sql"
	"errors"

	sq "github.com/Masterminds/squirrel"
)

var ErrContainerDisappeared = errors.New("container-disappeared-from-db")

type ContainerState string

const (
	ContainerStateCreating   = "creating"
	ContainerStateCreated    = "created"
	ContainerStateDestroying = "destroying"
)

type ContainerStage string

const (
	ContainerStageCheck = "check"
	ContainerStageGet   = "get"
	ContainerStageRun   = "run"
)

//go:generate counterfeiter . CreatingContainer

type CreatingContainer interface {
	ID() int
	Handle() string
	Created() (CreatedContainer, error)
}

type creatingContainer struct {
	id         int
	handle     string
	workerName string
	conn       Conn
}

func (container *creatingContainer) ID() int        { return container.id }
func (container *creatingContainer) Handle() string { return container.handle }

func (container *creatingContainer) Created() (CreatedContainer, error) {
	tx, err := container.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	rows, err := psql.Update("containers").
		Set("state", ContainerStateCreated).
		Where(sq.Eq{
			"id":    container.id,
			"state": ContainerStateCreating,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return nil, err
	}

	if affected == 0 {
		return nil, ErrContainerDisappeared
	}

	return &createdContainer{
		id:         container.id,
		handle:     container.handle,
		workerName: container.workerName,
		hijacked:   false,
		conn:       container.conn,
	}, nil
}

//go:generate counterfeiter . CreatedContainer

type CreatedContainer interface {
	ID() int
	Handle() string
	Discontinue() (DestroyingContainer, error)
	Destroying() (DestroyingContainer, error)
	WorkerName() string
	MarkAsHijacked() error
}

type createdContainer struct {
	id         int
	handle     string
	workerName string
	hijacked   bool
	conn       Conn
}

func (container *createdContainer) ID() int            { return container.id }
func (container *createdContainer) Handle() string     { return container.handle }
func (container *createdContainer) WorkerName() string { return container.workerName }

func (container *createdContainer) Destroying() (DestroyingContainer, error) {
	tx, err := container.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	var isDiscontinued bool

	err = psql.Update("containers").
		Set("state", ContainerStateDestroying).
		Where(sq.Eq{
			"id":    container.id,
			"state": ContainerStateCreated,
		}).
		Suffix("RETURNING discontinued").
		RunWith(tx).
		QueryRow().
		Scan(&isDiscontinued)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrContainerDisappeared
		}

		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &destroyingContainer{
		id:             container.id,
		handle:         container.handle,
		workerName:     container.workerName,
		isDiscontinued: isDiscontinued,
		conn:           container.conn,
	}, nil
}

func (container *createdContainer) Discontinue() (DestroyingContainer, error) {
	tx, err := container.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	rows, err := psql.Update("containers").
		Set("state", ContainerStateDestroying).
		Set("discontinued", true).
		Where(sq.Eq{
			"id":    container.id,
			"state": ContainerStateCreated,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return nil, err
	}

	if affected == 0 {
		return nil, ErrContainerDisappeared
	}

	return &destroyingContainer{
		id:             container.id,
		handle:         container.handle,
		workerName:     container.workerName,
		isDiscontinued: true,
		conn:           container.conn,
	}, nil
}

func (container *createdContainer) MarkAsHijacked() error {
	if container.hijacked {
		return nil
	}

	tx, err := container.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	rows, err := psql.Update("containers").
		Set("hijacked", true).
		Where(sq.Eq{
			"id":    container.id,
			"state": ContainerStateCreated,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return ErrContainerDisappeared
	}

	return nil
}

//go:generate counterfeiter . DestroyingContainer

type DestroyingContainer interface {
	Handle() string
	WorkerName() string
	Destroy() (bool, error)
	IsDiscontinued() bool
}

type destroyingContainer struct {
	id             int
	handle         string
	workerName     string
	isDiscontinued bool
	conn           Conn
}

func (container *destroyingContainer) Handle() string       { return container.handle }
func (container *destroyingContainer) WorkerName() string   { return container.workerName }
func (container *destroyingContainer) IsDiscontinued() bool { return container.isDiscontinued }

func (container *destroyingContainer) Destroy() (bool, error) {
	tx, err := container.conn.Begin()
	if err != nil {
		return false, err
	}

	defer tx.Rollback()

	rows, err := psql.Delete("containers").
		Where(sq.Eq{
			"id":    container.id,
			"state": ContainerStateDestroying,
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
		return false, ErrContainerDisappeared
	}

	return true, nil
}

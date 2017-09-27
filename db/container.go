package db

import (
	"database/sql"
	"errors"

	sq "github.com/Masterminds/squirrel"
)

var ErrContainerDisappeared = errors.New("container disappeared from db")

type ContainerState string

const (
	ContainerStateCreated    = "created"
	ContainerStateCreating   = "creating"
	ContainerStateDestroying = "destroying"
	ContainerStateFailed     = "failed"
)

//go:generate counterfeiter . Container

type Container interface {
	ID() int
	Handle() string
	WorkerName() string
	Metadata() ContainerMetadata
}

//go:generate counterfeiter . CreatingContainer

type CreatingContainer interface {
	Container

	Created() (CreatedContainer, error)
	Failed() (FailedContainer, error)
}

type creatingContainer struct {
	id         int
	handle     string
	workerName string
	metadata   ContainerMetadata
	conn       Conn
}

func newCreatingContainer(
	id int,
	handle string,
	workerName string,
	metadata ContainerMetadata,
	conn Conn,
) *creatingContainer {
	return &creatingContainer{
		id:         id,
		handle:     handle,
		workerName: workerName,
		metadata:   metadata,
		conn:       conn,
	}
}

func (container *creatingContainer) ID() int                     { return container.id }
func (container *creatingContainer) Handle() string              { return container.handle }
func (container *creatingContainer) WorkerName() string          { return container.workerName }
func (container *creatingContainer) Metadata() ContainerMetadata { return container.metadata }

func (container *creatingContainer) Created() (CreatedContainer, error) {
	rows, err := psql.Update("containers").
		Set("state", ContainerStateCreated).
		Where(sq.And{
			sq.Eq{"id": container.id},
			sq.Or{
				sq.Eq{"state": string(ContainerStateCreating)},
				sq.Eq{"state": string(ContainerStateCreated)},
			},
		}).
		RunWith(container.conn).
		Exec()
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

	return newCreatedContainer(
		container.id,
		container.handle,
		container.workerName,
		container.metadata,
		false,
		container.conn,
	), nil
}

func (container *creatingContainer) Failed() (FailedContainer, error) {
	rows, err := psql.Update("containers").
		Set("state", ContainerStateFailed).
		Where(sq.And{
			sq.Eq{"id": container.id},
			sq.Or{
				sq.Eq{"state": string(ContainerStateCreating)},
				sq.Eq{"state": string(ContainerStateFailed)},
			},
		}).
		RunWith(container.conn).
		Exec()
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

	return newFailedContainer(
		container.id,
		container.handle,
		container.workerName,
		container.metadata,
		container.conn,
	), nil
}

//go:generate counterfeiter . CreatedContainer

type CreatedContainer interface {
	Container

	Discontinue() (DestroyingContainer, error)
	Destroying() (DestroyingContainer, error)
	IsHijacked() bool
	MarkAsHijacked() error
}

type createdContainer struct {
	id         int
	handle     string
	workerName string
	metadata   ContainerMetadata

	hijacked bool

	conn Conn
}

func newCreatedContainer(
	id int,
	handle string,
	workerName string,
	metadata ContainerMetadata,
	hijacked bool,
	conn Conn,
) *createdContainer {
	return &createdContainer{
		id:         id,
		handle:     handle,
		workerName: workerName,
		metadata:   metadata,
		hijacked:   hijacked,
		conn:       conn,
	}
}

func (container *createdContainer) ID() int                     { return container.id }
func (container *createdContainer) Handle() string              { return container.handle }
func (container *createdContainer) WorkerName() string          { return container.workerName }
func (container *createdContainer) Metadata() ContainerMetadata { return container.metadata }

func (container *createdContainer) IsHijacked() bool { return container.hijacked }

func (container *createdContainer) Destroying() (DestroyingContainer, error) {
	var isDiscontinued bool

	err := psql.Update("containers").
		Set("state", ContainerStateDestroying).
		Where(sq.And{
			sq.Eq{"id": container.id},
			sq.Or{
				sq.Eq{"state": string(ContainerStateDestroying)},
				sq.Eq{"state": string(ContainerStateCreated)},
			},
		}).
		Suffix("RETURNING discontinued").
		RunWith(container.conn).
		QueryRow().
		Scan(&isDiscontinued)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrContainerDisappeared
		}

		return nil, err
	}

	return newDestroyingContainer(
		container.id,
		container.handle,
		container.workerName,
		container.metadata,
		isDiscontinued,
		container.conn,
	), nil
}

func (container *createdContainer) Discontinue() (DestroyingContainer, error) {
	rows, err := psql.Update("containers").
		Set("state", ContainerStateDestroying).
		Set("discontinued", true).
		Where(sq.And{
			sq.Eq{"id": container.id},
			sq.Or{
				sq.Eq{"state": string(ContainerStateDestroying)},
				sq.Eq{"state": string(ContainerStateCreated)},
			},
		}).
		RunWith(container.conn).
		Exec()
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

	return newDestroyingContainer(
		container.id,
		container.handle,
		container.workerName,
		container.metadata,
		true,
		container.conn,
	), nil
}

func (container *createdContainer) MarkAsHijacked() error {
	if container.hijacked {
		return nil
	}

	rows, err := psql.Update("containers").
		Set("hijacked", true).
		Where(sq.Eq{
			"id":    container.id,
			"state": ContainerStateCreated,
		}).
		RunWith(container.conn).
		Exec()
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
	Container

	Destroy() (bool, error)
	IsDiscontinued() bool
}

type destroyingContainer struct {
	id         int
	handle     string
	workerName string
	metadata   ContainerMetadata

	isDiscontinued bool

	conn Conn
}

func newDestroyingContainer(
	id int,
	handle string,
	workerName string,
	metadata ContainerMetadata,
	isDiscontinued bool,
	conn Conn,
) *destroyingContainer {
	return &destroyingContainer{
		id:             id,
		handle:         handle,
		workerName:     workerName,
		metadata:       metadata,
		isDiscontinued: isDiscontinued,
		conn:           conn,
	}
}

func (container *destroyingContainer) ID() int                     { return container.id }
func (container *destroyingContainer) Handle() string              { return container.handle }
func (container *destroyingContainer) WorkerName() string          { return container.workerName }
func (container *destroyingContainer) Metadata() ContainerMetadata { return container.metadata }

func (container *destroyingContainer) IsDiscontinued() bool { return container.isDiscontinued }

func (container *destroyingContainer) Destroy() (bool, error) {
	rows, err := psql.Delete("containers").
		Where(sq.Eq{
			"id":    container.id,
			"state": ContainerStateDestroying,
		}).
		RunWith(container.conn).
		Exec()
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

//go:generate counterfeiter . FailedContainer

type FailedContainer interface {
	Container

	Destroy() (bool, error)
}

type failedContainer struct {
	id         int
	handle     string
	workerName string
	metadata   ContainerMetadata
	conn       Conn
}

func newFailedContainer(
	id int,
	handle string,
	workerName string,
	metadata ContainerMetadata,
	conn Conn,
) *failedContainer {
	return &failedContainer{
		id:         id,
		handle:     handle,
		workerName: workerName,
		metadata:   metadata,
		conn:       conn,
	}
}

func (container *failedContainer) ID() int                     { return container.id }
func (container *failedContainer) Handle() string              { return container.handle }
func (container *failedContainer) WorkerName() string          { return container.workerName }
func (container *failedContainer) Metadata() ContainerMetadata { return container.metadata }

func (container *failedContainer) Destroy() (bool, error) {
	rows, err := psql.Delete("containers").
		Where(sq.Eq{
			"id":    container.id,
			"state": ContainerStateFailed,
		}).
		RunWith(container.conn).
		Exec()
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

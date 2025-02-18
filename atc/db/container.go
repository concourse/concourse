package db

import (
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
)

var ErrContainerDisappeared = errors.New("container disappeared from db")

type ContainerState string

//counterfeiter:generate . Container
type Container interface {
	ID() int
	State() string
	Handle() string
	WorkerName() string
	Metadata() ContainerMetadata
}

//counterfeiter:generate . CreatingContainer
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
	conn       DbConn
}

func newCreatingContainer(
	id int,
	handle string,
	workerName string,
	metadata ContainerMetadata,
	conn DbConn,
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
func (container *creatingContainer) State() string               { return atc.ContainerStateCreating }
func (container *creatingContainer) Handle() string              { return container.handle }
func (container *creatingContainer) WorkerName() string          { return container.workerName }
func (container *creatingContainer) Metadata() ContainerMetadata { return container.metadata }

func (container *creatingContainer) Created() (CreatedContainer, error) {
	rows, err := psql.Update("containers").
		Set("state", atc.ContainerStateCreated).
		Where(sq.And{
			sq.Eq{"id": container.id},
			sq.Or{
				sq.Eq{"state": string(atc.ContainerStateCreating)},
				sq.Eq{"state": string(atc.ContainerStateCreated)},
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
		time.Time{},
		container.conn,
	), nil
}

func (container *creatingContainer) Failed() (FailedContainer, error) {
	rows, err := psql.Update("containers").
		Set("state", atc.ContainerStateFailed).
		Where(sq.And{
			sq.Eq{"id": container.id},
			sq.Or{
				sq.Eq{"state": string(atc.ContainerStateCreating)},
				sq.Eq{"state": string(atc.ContainerStateFailed)},
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

//counterfeiter:generate . CreatedContainer
type CreatedContainer interface {
	Container

	Destroying() (DestroyingContainer, error)
	LastHijack() time.Time
	UpdateLastHijack() error
}

type createdContainer struct {
	id         int
	handle     string
	workerName string
	metadata   ContainerMetadata

	lastHijack time.Time

	conn DbConn
}

func newCreatedContainer(
	id int,
	handle string,
	workerName string,
	metadata ContainerMetadata,
	lastHijack time.Time,
	conn DbConn,
) *createdContainer {
	return &createdContainer{
		id:         id,
		handle:     handle,
		workerName: workerName,
		metadata:   metadata,
		lastHijack: lastHijack,
		conn:       conn,
	}
}

func (container *createdContainer) ID() int                     { return container.id }
func (container *createdContainer) State() string               { return atc.ContainerStateCreated }
func (container *createdContainer) Handle() string              { return container.handle }
func (container *createdContainer) WorkerName() string          { return container.workerName }
func (container *createdContainer) Metadata() ContainerMetadata { return container.metadata }

func (container *createdContainer) LastHijack() time.Time { return container.lastHijack }

func (container *createdContainer) Destroying() (DestroyingContainer, error) {

	rows, err := psql.Update("containers").
		Set("state", atc.ContainerStateDestroying).
		Where(sq.And{
			sq.Eq{"id": container.id},
			sq.Or{
				sq.Eq{"state": string(atc.ContainerStateDestroying)},
				sq.Eq{"state": string(atc.ContainerStateCreated)},
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
		container.conn,
	), nil
}

func (container *createdContainer) UpdateLastHijack() error {

	rows, err := psql.Update("containers").
		Set("last_hijack", sq.Expr("now()")).
		Where(sq.Eq{
			"id":    container.id,
			"state": atc.ContainerStateCreated,
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

//counterfeiter:generate . DestroyingContainer
type DestroyingContainer interface {
	Container

	Destroy() (bool, error)
}

type destroyingContainer struct {
	id         int
	handle     string
	workerName string
	metadata   ContainerMetadata

	conn DbConn
}

func newDestroyingContainer(
	id int,
	handle string,
	workerName string,
	metadata ContainerMetadata,
	conn DbConn,
) *destroyingContainer {
	return &destroyingContainer{
		id:         id,
		handle:     handle,
		workerName: workerName,
		metadata:   metadata,
		conn:       conn,
	}
}

func (container *destroyingContainer) ID() int                     { return container.id }
func (container *destroyingContainer) State() string               { return atc.ContainerStateDestroying }
func (container *destroyingContainer) Handle() string              { return container.handle }
func (container *destroyingContainer) WorkerName() string          { return container.workerName }
func (container *destroyingContainer) Metadata() ContainerMetadata { return container.metadata }

func (container *destroyingContainer) Destroy() (bool, error) {
	rows, err := psql.Delete("containers").
		Where(sq.Eq{
			"id":    container.id,
			"state": atc.ContainerStateDestroying,
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

//counterfeiter:generate . FailedContainer
type FailedContainer interface {
	Container

	Destroy() (bool, error)
}

type failedContainer struct {
	id         int
	handle     string
	workerName string
	metadata   ContainerMetadata
	conn       DbConn
}

func newFailedContainer(
	id int,
	handle string,
	workerName string,
	metadata ContainerMetadata,
	conn DbConn,
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
func (container *failedContainer) State() string               { return atc.ContainerStateFailed }
func (container *failedContainer) Handle() string              { return container.handle }
func (container *failedContainer) WorkerName() string          { return container.workerName }
func (container *failedContainer) Metadata() ContainerMetadata { return container.metadata }

func (container *failedContainer) Destroy() (bool, error) {
	rows, err := psql.Delete("containers").
		Where(sq.Eq{
			"id":    container.id,
			"state": atc.ContainerStateFailed,
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

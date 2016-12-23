package dbng

import (
	sq "github.com/Masterminds/squirrel"
)

type ContainerState string

const (
	ContainerStateCreating   = "creating"
	ContainerStateCreated    = "created"
	ContainerStateDestroying = "destroying"
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
		panic("TESTME")
		return nil, nil
	}

	return &createdContainer{
		id:         container.id,
		handle:     container.handle,
		workerName: container.workerName,
		conn:       container.conn,
	}, nil
}

//go:generate counterfeiter . CreatedContainer

type CreatedContainer interface {
	ID() int
	Destroying() (DestroyingContainer, error)
}

type createdContainer struct {
	id         int
	handle     string
	workerName string
	conn       Conn
}

func (container *createdContainer) ID() int { return container.id }

func (container *createdContainer) Destroying() (DestroyingContainer, error) {
	tx, err := container.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	rows, err := psql.Update("containers").
		Set("state", ContainerStateDestroying).
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
		panic("TESTME")
		return nil, nil
	}

	return &destroyingContainer{
		id:         container.id,
		handle:     container.handle,
		workerName: container.workerName,
		conn:       container.conn,
	}, nil
}

//go:generate counterfeiter . DestroyingContainer

type DestroyingContainer interface {
	Handle() string
	WorkerName() string
	Destroy() (bool, error)
}

type destroyingContainer struct {
	id         int
	handle     string
	workerName string
	conn       Conn
}

func (container *destroyingContainer) Handle() string     { return container.handle }
func (container *destroyingContainer) WorkerName() string { return container.workerName }

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
		panic("TESTME")
		return false, nil
	}

	return true, nil
}

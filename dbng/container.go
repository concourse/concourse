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

type CreatingContainer struct {
	ID int
}

func (container *CreatingContainer) Created(tx Tx, handle string) (*CreatedContainer, error) {
	rows, err := psql.Update("containers").
		Set("state", ContainerStateCreated).
		Set("handle", handle).
		Where(sq.Eq{
			"id":    container.ID,
			"state": ContainerStateCreating,
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

	return &CreatedContainer{
		ID: container.ID,
	}, nil
}

type CreatedContainer struct {
	ID int
}

func (container *CreatedContainer) Destroying(tx Tx) (*DestroyingContainer, error) {
	rows, err := psql.Update("containers").
		Set("state", ContainerStateCreated).
		Where(sq.Eq{
			"id":    container.ID,
			"state": ContainerStateDestroying,
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

	return &DestroyingContainer{
		ID: container.ID,
	}, nil
}

type DestroyingContainer struct {
	ID int
}

func (container *DestroyingContainer) Destroy(tx Tx) (bool, error) {
	rows, err := psql.Delete("containers").
		Where(sq.Eq{
			"id":    container.ID,
			"state": ContainerStateDestroying,
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

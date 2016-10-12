package dbng

import (
	"database/sql"

	"code.cloudfoundry.org/lager"

	sq "github.com/Masterminds/squirrel"
)

type ContainerState string

const (
	ContainerStateCreating   = "creating"
	ContainerStateCreated    = "created"
	ContainerStateDestroying = "destroying"
)

type CreatingContainer struct {
	ID   int
	conn Conn
}

func (container *CreatingContainer) Created(logger lager.Logger, handle string) (*CreatedContainer, error) {
	logger.Debug("creating-transaction", lager.Data{"stats": container.conn.Stats()})
	tx, err := container.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	logger.Debug("updating-containers")
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

	logger.Debug("commiting-transaction")

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	logger.Debug("checking-affected-rows")
	affected, err := rows.RowsAffected()
	if err != nil {
		return nil, err
	}

	if affected == 0 {
		logger.Debug("no-rows-affected")
		panic("TESTME")
		return nil, nil
	}

	logger.Debug("returning-created-container")
	return &CreatedContainer{
		ID:   container.ID,
		conn: container.conn,
	}, nil
}

type CreatedContainer struct {
	ID   int
	conn Conn
}

func (container *CreatedContainer) Volumes() ([]CreatedVolume, error) {
	query, args, err := psql.Select("v.id, v.handle, v.path, v.state, w.name, w.addr").
		From("volumes v").
		LeftJoin("workers w ON v.worker_name = w.name").
		Where(sq.Eq{
			"v.state": VolumeStateCreated,
		}).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := container.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	createdVolumes := []CreatedVolume{}

	for rows.Next() {
		var id int
		var handle string
		var path sql.NullString
		var state string
		var workerName string
		var workerAddress string

		err = rows.Scan(&id, &handle, &path, &state, &workerName, &workerAddress)
		if err != nil {
			return nil, err
		}

		var pathString string
		if path.Valid {
			pathString = path.String
		}

		createdVolumes = append(createdVolumes, &createdVolume{
			id:     id,
			handle: handle,
			path:   pathString,
			worker: &Worker{
				Name:       workerName,
				GardenAddr: workerAddress,
			},
			conn: container.conn,
		})
	}

	return createdVolumes, nil
}

func (container *CreatedContainer) Destroying() (*DestroyingContainer, error) {
	tx, err := container.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	rows, err := psql.Update("containers").
		Set("state", ContainerStateDestroying).
		Where(sq.Eq{
			"id":    container.ID,
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

	return &DestroyingContainer{
		ID:   container.ID,
		conn: container.conn,
	}, nil
}

type DestroyingContainer struct {
	ID   int
	conn Conn
}

func (container *DestroyingContainer) Destroy() (bool, error) {
	tx, err := container.conn.Begin()
	if err != nil {
		return false, err
	}

	defer tx.Rollback()

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

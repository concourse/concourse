package dbng

import sq "github.com/Masterminds/squirrel"

//go:generate counterfeiter . ContainerFactory

type ContainerFactory interface {
	FindContainersMarkedForDeletion() ([]DestroyingContainer, error)
	MarkContainersForDeletion() error
	FindHijackedContainersForDeletion() ([]CreatedContainer, error)
}

type containerFactory struct {
	conn Conn
}

func NewContainerFactory(conn Conn) ContainerFactory {
	return &containerFactory{
		conn: conn,
	}
}

type ContainerMetadata struct {
	Type string
	Name string
}

func (factory *containerFactory) FindContainersMarkedForDeletion() ([]DestroyingContainer, error) {
	query, args, err := psql.Select("id, handle, worker_name, discontinued").
		From("containers").
		Where(sq.Eq{
			"state": ContainerStateDestroying,
		}).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := factory.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var (
		results        []DestroyingContainer
		id             int
		handle         string
		workerName     string
		isDiscontinued bool
	)

	for rows.Next() {
		err := rows.Scan(&id, &handle, &workerName, &isDiscontinued)
		if err != nil {
			return nil, err
		}

		results = append(results, &destroyingContainer{
			id:             id,
			handle:         handle,
			workerName:     workerName,
			isDiscontinued: isDiscontinued,
			conn:           factory.conn,
		})
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return results, nil
}

func (factory *containerFactory) MarkContainersForDeletion() error {
	tx, err := factory.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = sq.Update("containers").
		Set("state", string(ContainerStateDestroying)).
		Where(containersToDeleteCondition).
		Where(sq.Eq{"hijacked": false}).
		PlaceholderFormat(sq.Dollar).
		RunWith(tx).
		Exec()

	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (factory *containerFactory) FindHijackedContainersForDeletion() ([]CreatedContainer, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	rows, err := sq.Select("id, handle, worker_name").
		From("containers").
		Where(containersToDeleteCondition).
		Where(sq.Eq{"hijacked": true}).
		Where(sq.Eq{"state": string(ContainerStateCreated)}).
		PlaceholderFormat(sq.Dollar).
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var (
		results    []CreatedContainer
		id         int
		handle     string
		workerName string
	)

	for rows.Next() {
		err := rows.Scan(&id, &handle, &workerName)
		if err != nil {
			return nil, err
		}

		results = append(results, &createdContainer{
			id:         id,
			handle:     handle,
			workerName: workerName,
			hijacked:   true,
			conn:       factory.conn,
		})
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return results, nil
}

var containersToDeleteCondition = sq.And{
	sq.Or{
		sq.Expr("(build_id IS NOT NULL AND build_id IN (SELECT id FROM builds WHERE interceptible = false))"),
		sq.Expr("(type = ? AND best_if_used_by < NOW())", string(ContainerStageCheck)),
		sq.Expr("(build_id IS NULL AND resource_config_id IS NULL AND resource_cache_id IS NULL)"),
		sq.Expr("(resource_config_id IS NOT NULL AND worker_base_resource_types_id IS NULL)"),
	},
	sq.Eq{"state": string(ContainerStateCreated)},
}

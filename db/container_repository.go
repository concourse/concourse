package db

import sq "github.com/Masterminds/squirrel"

//go:generate counterfeiter . ContainerRepository

type ContainerRepository interface {
	FindOrphanedContainers() ([]CreatingContainer, []CreatedContainer, []DestroyingContainer, error)
	FindFailedContainers() ([]FailedContainer, error)
}

type containerRepository struct {
	conn Conn
}

func NewContainerRepository(conn Conn) ContainerRepository {
	return &containerRepository{
		conn: conn,
	}
}

func (repository *containerRepository) FindOrphanedContainers() ([]CreatingContainer, []CreatedContainer, []DestroyingContainer, error) {
	query, args, err := selectContainers("c").
		LeftJoin("builds b ON b.id = c.build_id").
		LeftJoin("containers icc ON icc.id = c.image_check_container_id").
		LeftJoin("containers igc ON igc.id = c.image_get_container_id").
		Where(sq.Or{
			sq.Eq{
				"c.build_id":                                nil,
				"c.image_check_container_id":                nil,
				"c.image_get_container_id":                  nil,
				"c.worker_resource_config_check_session_id": nil,
			},
			sq.And{
				sq.NotEq{"c.build_id": nil},
				sq.Eq{"b.interceptible": false},
			},
			sq.And{
				sq.NotEq{"c.image_check_container_id": nil},
				sq.NotEq{"icc.state": ContainerStateCreating},
			},
			sq.And{
				sq.NotEq{"c.image_get_container_id": nil},
				sq.NotEq{"igc.state": ContainerStateCreating},
			},
		}).
		ToSql()
	if err != nil {
		return nil, nil, nil, err
	}

	rows, err := repository.conn.Query(query, args...)
	if err != nil {
		return nil, nil, nil, err
	}

	defer Close(rows)

	creatingContainers := []CreatingContainer{}
	createdContainers := []CreatedContainer{}
	destroyingContainers := []DestroyingContainer{}

	var (
		creatingContainer   CreatingContainer
		createdContainer    CreatedContainer
		destroyingContainer DestroyingContainer
	)

	for rows.Next() {
		creatingContainer, createdContainer, destroyingContainer, _, err = scanContainer(rows, repository.conn)
		if err != nil {
			return nil, nil, nil, err
		}

		if creatingContainer != nil {
			creatingContainers = append(creatingContainers, creatingContainer)
		}

		if createdContainer != nil {
			createdContainers = append(createdContainers, createdContainer)
		}

		if destroyingContainer != nil {
			destroyingContainers = append(destroyingContainers, destroyingContainer)
		}
	}

	err = rows.Err()
	if err != nil {
		return nil, nil, nil, err
	}

	return creatingContainers, createdContainers, destroyingContainers, nil
}

func selectContainers(asOptional ...string) sq.SelectBuilder {
	columns := []string{"id", "handle", "worker_name", "hijacked", "discontinued", "state"}
	columns = append(columns, containerMetadataColumns...)

	table := "containers"
	if len(asOptional) > 0 {
		as := asOptional[0]

		for i, c := range columns {
			columns[i] = as + "." + c
		}

		table += " " + as
	}

	return psql.Select(columns...).From(table)
}

func scanContainer(row sq.RowScanner, conn Conn) (CreatingContainer, CreatedContainer, DestroyingContainer, FailedContainer, error) {
	var (
		id             int
		handle         string
		workerName     string
		isDiscontinued bool
		isHijacked     bool
		state          string

		metadata ContainerMetadata
	)

	columns := []interface{}{&id, &handle, &workerName, &isHijacked, &isDiscontinued, &state}
	columns = append(columns, metadata.ScanTargets()...)

	err := row.Scan(columns...)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	switch state {
	case ContainerStateCreating:
		return newCreatingContainer(
			id,
			handle,
			workerName,
			metadata,
			conn,
		), nil, nil, nil, nil
	case ContainerStateCreated:
		return nil, newCreatedContainer(
			id,
			handle,
			workerName,
			metadata,
			isHijacked,
			conn,
		), nil, nil, nil
	case ContainerStateDestroying:
		return nil, nil, newDestroyingContainer(
			id,
			handle,
			workerName,
			metadata,
			isDiscontinued,
			conn,
		), nil, nil
	case ContainerStateFailed:
		return nil, nil, nil, newFailedContainer(
			id,
			handle,
			workerName,
			metadata,
			conn,
		), nil
	}

	return nil, nil, nil, nil, nil
}

func (repository *containerRepository) FindFailedContainers() ([]FailedContainer, error) {
	var failedContainers []FailedContainer

	query, args, _ := selectContainers("c").
		LeftJoin("builds b ON b.id = c.build_id").
		LeftJoin("containers icc ON icc.id = c.image_check_container_id").
		LeftJoin("containers igc ON igc.id = c.image_get_container_id").
		Where(sq.Eq{"c.state": ContainerStateFailed}).
		ToSql()

	rows, err := repository.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	for rows.Next() {
		_, _, _, failedContainer, err := scanContainer(rows, repository.conn)
		if err != nil {
			return nil, err
		}

		if failedContainer != nil {
			failedContainers = append(failedContainers, failedContainer)
		}
	}

	return failedContainers, nil
}

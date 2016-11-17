package dbng

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"

	"github.com/concourse/atc"
)

type ContainerFactory struct {
	conn Conn
}

func NewContainerFactory(conn Conn) *ContainerFactory {
	return &ContainerFactory{
		conn: conn,
	}
}

type ContainerMetadata struct {
	Type string
	Name string
}

func (factory *ContainerFactory) CreateResourceCheckContainer(
	worker *Worker,
	resourceConfig *UsedResourceConfig,
	stepName string,
) (*CreatingContainer, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	var containerID int
	err = psql.Insert("containers").
		Columns(
			"worker_name",
			"resource_config_id",
			"type",
			"step_name",
		).
		Values(
			worker.Name,
			resourceConfig.ID,
			"check",
			stepName,
		).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&containerID)
	if err != nil {
		// TODO: explicitly handle fkey constraint
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &CreatingContainer{
		ID:   containerID,
		conn: factory.conn,
	}, nil
}

func (factory *ContainerFactory) ContainerCreated(
	container *CreatingContainer,
	gardenHandle string,
) (*CreatedContainer, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	rows, err := psql.Update("containers").
		Set("state", ContainerStateCreated).
		Set("handle", gardenHandle).
		Where(sq.Eq{
			"id":    container.ID,
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

	return &CreatedContainer{
		ID:   container.ID,
		conn: factory.conn,
	}, nil
}

func (factory *ContainerFactory) CreateResourceGetContainer(
	worker *Worker,
	resourceCache *UsedResourceCache,
	stepName string,
) (*CreatingContainer, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	var containerID int
	err = psql.Insert("containers").
		Columns(
			"worker_name",
			"resource_cache_id",
			"type",
			"step_name",
		).
		Values(
			worker.Name,
			resourceCache.ID,
			"get",
			stepName,
		).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&containerID)
	if err != nil {
		// TODO: explicitly handle fkey constraint
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &CreatingContainer{
		ID:   containerID,
		conn: factory.conn,
	}, nil
}

func (factory *ContainerFactory) CreateBuildContainer(
	worker *Worker,
	build *Build,
	planID atc.PlanID,
	meta ContainerMetadata,
) (*CreatingContainer, error) {
	return factory.createPlanContainer(worker, build, planID, meta)
}

func (factory *ContainerFactory) FindContainer(
	handle string,
) (*CreatedContainer, bool, error) {
	return factory.findContainer(handle)
}

func (factory *ContainerFactory) createPlanContainer(
	worker *Worker,
	build *Build,
	planID atc.PlanID,
	meta ContainerMetadata,
) (*CreatingContainer, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	var containerID int
	err = psql.Insert("containers").
		// TODO: should metadata just be JSON?
		Columns(
			"worker_name",
			"build_id",
			"plan_id",
			"type",
			"step_name",
		).
		Values(
			worker.Name,
			build.ID,
			string(planID),
			meta.Type,
			meta.Name,
		).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&containerID)
	if err != nil {
		// TODO: explicitly handle fkey constraint
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &CreatingContainer{
		ID:   containerID,
		conn: factory.conn,
	}, nil
}

func (factory *ContainerFactory) findContainer(handle string) (*CreatedContainer, bool, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer tx.Rollback()

	var containerID int
	err = psql.Select("id").
		From("containers").
		Where(sq.Eq{
			"state":  ContainerStateCreated,
			"handle": handle,
		}).
		RunWith(tx).
		QueryRow().
		Scan(&containerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	return &CreatedContainer{
		ID:   containerID,
		conn: factory.conn,
	}, true, nil
}

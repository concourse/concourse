package dbng

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"

	"github.com/concourse/atc"
	"github.com/nu7hatch/gouuid"
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

func (factory *ContainerFactory) FindOrCreateResourceCheckContainer(
	worker *Worker,
	resourceConfig *UsedResourceConfig,
	stepName string,
) (*CreatingContainer, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	handle, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	var containerID int
	err = psql.Insert("containers").
		Columns(
			"worker_name",
			"resource_config_id",
			"type",
			"step_name",
			"handle",
		).
		Values(
			worker.Name,
			resourceConfig.ID,
			"check",
			stepName,
			handle.String(),
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
		ID:         containerID,
		Handle:     handle.String(),
		WorkerName: worker.Name,
		conn:       factory.conn,
	}, nil
}

func (factory *ContainerFactory) ContainerCreated(
	container *CreatingContainer,
) (*CreatedContainer, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	rows, err := psql.Update("containers").
		Set("state", ContainerStateCreated).
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
		ID:         container.ID,
		Handle:     container.Handle,
		WorkerName: container.WorkerName,
		conn:       factory.conn,
	}, nil
}

func (factory *ContainerFactory) ContainerDestroying(
	container *CreatedContainer,
) (*DestroyingContainer, error) {
	tx, err := factory.conn.Begin()
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
		ID:         container.ID,
		Handle:     container.Handle,
		WorkerName: container.WorkerName,
		conn:       factory.conn,
	}, nil
}

func (factory *ContainerFactory) FindContainersMarkedForDeletion() ([]*DestroyingContainer, error) {
	query, args, err := psql.Select("id, handle, worker_name").
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
		results    []*DestroyingContainer
		id         int
		handle     string
		workerName string
	)

	for rows.Next() {
		err := rows.Scan(&id, &handle, &workerName)
		if err != nil {
			return nil, err
		}

		results = append(results, &DestroyingContainer{
			ID:         id,
			Handle:     handle,
			WorkerName: workerName,
		})
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return results, nil
}

func (factory *ContainerFactory) MarkBuildContainersForDeletion() error {
	tx, err := factory.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec(`WITH
	    latest_builds AS (
	        SELECT COALESCE(MAX(b.id)) AS build_id
	        FROM builds b, jobs j
	        WHERE b.job_id = j.id
	        AND b.completed
	    ),
	    builds_to_keep AS (
	        SELECT id FROM builds
	        WHERE (
	            (status = $2 OR status = $3 OR status = $4)
	            AND id IN (SELECT build_id FROM latest_builds)
	        ) OR (
	            NOT completed
	        )
	    )
		UPDATE containers SET state = $1
		WHERE build_id IS NOT NULL AND build_id NOT IN (SELECT id FROM builds_to_keep)`,
		string(ContainerStateDestroying),
		string(BuildStatusAborted),
		string(BuildStatusErrored),
		string(BuildStatusFailed),
	)

	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (factory *ContainerFactory) ContainerDestroy(
	container *DestroyingContainer,
) (bool, error) {
	tx, err := factory.conn.Begin()
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

	handle, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	var containerID int
	err = psql.Insert("containers").
		Columns(
			"worker_name",
			"resource_cache_id",
			"type",
			"step_name",
			"handle",
		).
		Values(
			worker.Name,
			resourceCache.ID,
			"get",
			stepName,
			handle.String(),
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
		ID:         containerID,
		Handle:     handle.String(),
		WorkerName: worker.Name,
		conn:       factory.conn,
	}, nil
}

func (factory *ContainerFactory) FindOrCreateBuildContainer(
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

	handle, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	var containerID int
	err = psql.Insert("containers").
		// TODO: should metadata just be JSON?
		Columns(
			"worker_name",
			"build_id",
			"plan_id",
			"type",
			"step_name",
			"handle",
		).
		Values(
			worker.Name,
			build.ID,
			string(planID),
			meta.Type,
			meta.Name,
			handle.String(),
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
		ID:         containerID,
		Handle:     handle.String(),
		WorkerName: worker.Name,
		conn:       factory.conn,
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

package dbng

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"

	"github.com/concourse/atc"
	"github.com/nu7hatch/gouuid"
)

//go:generate counterfeiter . ContainerFactory

type ContainerFactory interface {
	FindContainerByHandle(string) (CreatedContainer, bool, error)

	FindResourceCheckContainer(*Worker, *UsedResourceConfig) (CreatingContainer, CreatedContainer, error)
	CreateResourceCheckContainer(*Worker, *UsedResourceConfig) (CreatingContainer, error)

	FindResourceGetContainer(*Worker, *UsedResourceCache, string) (CreatingContainer, CreatedContainer, error)
	CreateResourceGetContainer(*Worker, *UsedResourceCache, string) (CreatingContainer, error)

	FindBuildContainer(*Worker, *Build, atc.PlanID, ContainerMetadata) (CreatingContainer, CreatedContainer, error)
	CreateBuildContainer(*Worker, *Build, atc.PlanID, ContainerMetadata) (CreatingContainer, error)

	FindContainersMarkedForDeletion() ([]DestroyingContainer, error)
	MarkBuildContainersForDeletion() error
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

func (factory *containerFactory) FindResourceCheckContainer(
	worker *Worker,
	resourceConfig *UsedResourceConfig,
) (CreatingContainer, CreatedContainer, error) {
	return factory.findContainer(map[string]interface{}{
		"worker_name":        worker.Name,
		"resource_config_id": resourceConfig.ID,
	})
}

func (factory *containerFactory) CreateResourceCheckContainer(
	worker *Worker,
	resourceConfig *UsedResourceConfig,
) (CreatingContainer, error) {
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
			"",
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

	return &creatingContainer{
		id:         containerID,
		handle:     handle.String(),
		workerName: worker.Name,
		conn:       factory.conn,
	}, nil
}

func (factory *containerFactory) FindContainersMarkedForDeletion() ([]DestroyingContainer, error) {
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
		results    []DestroyingContainer
		id         int
		handle     string
		workerName string
	)

	for rows.Next() {
		err := rows.Scan(&id, &handle, &workerName)
		if err != nil {
			return nil, err
		}

		results = append(results, &destroyingContainer{
			id:         id,
			handle:     handle,
			workerName: workerName,
			conn:       factory.conn,
		})
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return results, nil
}

func (factory *containerFactory) MarkBuildContainersForDeletion() error {
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

func (factory *containerFactory) FindResourceGetContainer(
	worker *Worker,
	resourceCache *UsedResourceCache,
	stepName string,
) (CreatingContainer, CreatedContainer, error) {
	return factory.findContainer(map[string]interface{}{
		"worker_name":       worker.Name,
		"resource_cache_id": resourceCache.ID,
		"step_name":         stepName,
	})
}

func (factory *containerFactory) CreateResourceGetContainer(
	worker *Worker,
	resourceCache *UsedResourceCache,
	stepName string,
) (CreatingContainer, error) {
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

	return &creatingContainer{
		id:         containerID,
		handle:     handle.String(),
		workerName: worker.Name,
		conn:       factory.conn,
	}, nil
}

func (factory *containerFactory) FindBuildContainer(
	worker *Worker,
	build *Build,
	planID atc.PlanID,
	meta ContainerMetadata,
) (CreatingContainer, CreatedContainer, error) {
	return factory.findContainer(map[string]interface{}{
		"worker_name": worker.Name,
		"build_id":    build.ID,
		"plan_id":     string(planID),
		"type":        meta.Type,
		"step_name":   meta.Name,
	})
}

func (factory *containerFactory) CreateBuildContainer(
	worker *Worker,
	build *Build,
	planID atc.PlanID,
	meta ContainerMetadata,
) (CreatingContainer, error) {
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

	return &creatingContainer{
		id:         containerID,
		handle:     handle.String(),
		workerName: worker.Name,
		conn:       factory.conn,
	}, nil
}

func (factory *containerFactory) FindContainerByHandle(
	handle string,
) (CreatedContainer, bool, error) {
	_, createdContainer, err := factory.findContainer(map[string]interface{}{
		"handle": handle,
	})
	if err != nil {
		return nil, false, err
	}

	if createdContainer != nil {
		return createdContainer, true, nil
	}

	return nil, false, nil
}

func (factory *containerFactory) findContainer(columns map[string]interface{}) (CreatingContainer, CreatedContainer, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, nil, err
	}

	defer tx.Rollback()

	whereClause := sq.Eq{}
	for name, value := range columns {
		whereClause[name] = value
	}

	var containerID int
	var workerName string
	var state string
	var handle string
	err = psql.Select("id, worker_name, state, handle").
		From("containers").
		Where(whereClause).
		RunWith(tx).
		QueryRow().
		Scan(&containerID, &workerName, &state, &handle)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, nil, err
	}

	switch state {
	case ContainerStateCreated:
		return nil, &createdContainer{
			id:         containerID,
			handle:     handle,
			workerName: workerName,
			conn:       factory.conn,
		}, nil
	case ContainerStateCreating:
		return &creatingContainer{
			id:         containerID,
			handle:     handle,
			workerName: workerName,
			conn:       factory.conn,
		}, nil, nil
	}

	return nil, nil, nil
}

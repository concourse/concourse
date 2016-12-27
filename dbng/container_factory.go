package dbng

import (
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

func (factory *ContainerFactory) FindOrCreateBuildContainer(
	worker *Worker,
	build *Build,
	planID atc.PlanID,
	meta ContainerMetadata,
) error {
	return factory.createPlanContainer(worker, build, planID, meta)
}

func (factory *ContainerFactory) createPlanContainer(
	worker *Worker,
	build *Build,
	planID atc.PlanID,
	meta ContainerMetadata,
) error {
	tx, err := factory.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	handle, err := uuid.NewV4()
	if err != nil {
		return err
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
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

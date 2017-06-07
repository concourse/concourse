package db

import (
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
)

//go:generate counterfeiter . ContainerOwner

// ContainerOwner designates the data the container should reference that
// identifies its lifecycle. When the owner goes away, the container should
// be garbage collected.
type ContainerOwner interface {
	Find(conn Conn) (sq.Eq, bool, error)
	Create(tx Tx, workerName string) (map[string]interface{}, error)
}

// NewImageCheckContainerOwner references a container whose image resource this
// container is checking. When the referenced container transitions to another
// state, or disappears, the container can be removed.
func NewImageCheckContainerOwner(
	container CreatingContainer,
) ContainerOwner {
	return imageCheckContainerOwner{
		Container: container,
	}
}

type imageCheckContainerOwner struct {
	Container CreatingContainer
}

func (c imageCheckContainerOwner) Find(Conn) (sq.Eq, bool, error) {
	return sq.Eq(c.sqlMap()), true, nil
}

func (c imageCheckContainerOwner) Create(Tx, string) (map[string]interface{}, error) {
	return c.sqlMap(), nil
}

func (c imageCheckContainerOwner) sqlMap() map[string]interface{} {
	return map[string]interface{}{
		"image_check_container_id": c.Container.ID(),
	}
}

// NewImageGetContainerOwner references a container whose image resource this
// container is fetching. When the referenced container transitions to another
// state, or disappears, the container can be removed.
func NewImageGetContainerOwner(
	container CreatingContainer,
) ContainerOwner {
	return imageGetContainerOwner{
		Container: container,
	}
}

type imageGetContainerOwner struct {
	Container CreatingContainer
}

func (c imageGetContainerOwner) Find(Conn) (sq.Eq, bool, error) {
	return sq.Eq(c.sqlMap()), true, nil
}

func (c imageGetContainerOwner) Create(Tx, string) (map[string]interface{}, error) {
	return c.sqlMap(), nil
}

func (c imageGetContainerOwner) sqlMap() map[string]interface{} {
	return map[string]interface{}{
		"image_get_container_id": c.Container.ID(),
	}
}

// NewBuildStepContainerOwner references a step within a build. When the build
// becomes non-interceptible or disappears, the container can be removed.
func NewBuildStepContainerOwner(
	buildID int,
	planID atc.PlanID,
) ContainerOwner {
	return buildStepContainerOwner{
		BuildID: buildID,
		PlanID:  planID,
	}
}

type buildStepContainerOwner struct {
	BuildID int
	PlanID  atc.PlanID
}

func (c buildStepContainerOwner) Find(Conn) (sq.Eq, bool, error) {
	return sq.Eq(c.sqlMap()), true, nil
}

func (c buildStepContainerOwner) Create(Tx, string) (map[string]interface{}, error) {
	return c.sqlMap(), nil
}

func (c buildStepContainerOwner) sqlMap() map[string]interface{} {
	return map[string]interface{}{
		"build_id": c.BuildID,
		"plan_id":  c.PlanID,
	}
}

// NewResourceConfigCheckSessionContainerOwner references a resource config and
// worker base resource type, with an expiry. When the resource config or
// worker base resource type disappear, or the expiry is reached, the container
// can be removed.
func NewResourceConfigCheckSessionContainerOwner(
	resourceConfig *UsedResourceConfig,
) ContainerOwner {
	return resourceConfigCheckSessionContainerOwner{
		UsedResourceConfig: resourceConfig,
	}
}

type resourceConfigCheckSessionContainerOwner struct {
	UsedResourceConfig *UsedResourceConfig
}

func (c resourceConfigCheckSessionContainerOwner) Find(conn Conn) (sq.Eq, bool, error) {
	var id int
	err := psql.Select("id").
		From("resource_config_check_sessions").
		Where(sq.And{
			sq.Eq{"resource_config_id": c.UsedResourceConfig.ID},
			sq.Expr("expires_at > NOW()"),
		}).
		RunWith(conn).
		QueryRow().
		Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}

		return nil, false, err
	}

	return sq.Eq{
		"resource_config_check_session_id": id,
	}, true, nil
}

func (c resourceConfigCheckSessionContainerOwner) Create(tx Tx, workerName string) (map[string]interface{}, error) {
	var wbrtID int
	err := psql.Select("id").
		From("worker_base_resource_types").
		Where(sq.Eq{
			"worker_name":           workerName,
			"base_resource_type_id": c.UsedResourceConfig.OriginBaseResourceType().ID,
		}).
		RunWith(tx).
		QueryRow().
		Scan(&wbrtID)
	if err != nil {
		return nil, err
	}

	var rccsID int
	err = psql.Select("id").
		From("resource_config_check_sessions").
		Where(sq.Eq{
			"resource_config_id":           c.UsedResourceConfig.ID,
			"worker_base_resource_type_id": wbrtID,
		}).
		RunWith(tx).
		QueryRow().
		Scan(&rccsID)
	if err != nil {
		if err == sql.ErrNoRows {
			var biub time.Time
			err = psql.Select("NOW() + LEAST(GREATEST('5 minutes'::interval, NOW() - to_timestamp(w.start_time)), '1 hour'::interval)").
				From("workers w").
				Where(sq.Eq{"w.name": workerName}).
				RunWith(tx).
				QueryRow().
				Scan(&biub)
			if err != nil {
				return nil, err
			}

			err = psql.Insert("resource_config_check_sessions").
				SetMap(map[string]interface{}{
					"resource_config_id":           c.UsedResourceConfig.ID,
					"worker_base_resource_type_id": wbrtID,
					"expires_at":                   biub,
				}).
				Suffix("RETURNING id").
				RunWith(tx).
				QueryRow().
				Scan(&rccsID)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return sq.Eq{
		"resource_config_check_session_id": rccsID,
	}, nil
}

package db

import (
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
)

//counterfeiter:generate . ContainerOwner

// ContainerOwner designates the data the container should reference that
// identifies its lifecycle. When the owner goes away, the container should
// be garbage collected.
type ContainerOwner interface {
	Find(conn DbConn) (sq.Eq, bool, error)
	Create(tx Tx, workerName string) (map[string]interface{}, error)
}

// NewBuildStepContainerOwner references a step within a build. When the build
// becomes non-interceptible or disappears, the container can be removed.
func NewBuildStepContainerOwner(
	buildID int,
	planID atc.PlanID,
	teamID int,
) ContainerOwner {
	return buildStepContainerOwner{
		BuildID: buildID,
		PlanID:  planID,
		TeamID:  teamID,
	}
}

type buildStepContainerOwner struct {
	BuildID int
	PlanID  atc.PlanID
	TeamID  int
}

func (c buildStepContainerOwner) Find(DbConn) (sq.Eq, bool, error) {
	return sq.Eq(c.sqlMap()), true, nil
}

func (c buildStepContainerOwner) Create(Tx, string) (map[string]interface{}, error) {
	return c.sqlMap(), nil
}

func (c buildStepContainerOwner) sqlMap() map[string]interface{} {
	return map[string]interface{}{
		"build_id": c.BuildID,
		"plan_id":  c.PlanID,
		"team_id":  c.TeamID,
	}
}

// NewResourceConfigCheckSessionContainerOwner references a resource config and
// worker base resource type, with an expiry. When the resource config or
// worker base resource type disappear, or the expiry is reached, the container
// can be removed.
func NewResourceConfigCheckSessionContainerOwner(
	resourceConfigID int,
	baseResourceTypeID int,
	expiries ContainerOwnerExpiries,
) ContainerOwner {
	return resourceConfigCheckSessionContainerOwner{
		resourceConfigID:   resourceConfigID,
		baseResourceTypeID: baseResourceTypeID,
		expiries:           expiries,
	}
}

type resourceConfigCheckSessionContainerOwner struct {
	resourceConfigID   int
	baseResourceTypeID int
	expiries           ContainerOwnerExpiries
}

type ContainerOwnerExpiries struct {
	Min time.Duration
	Max time.Duration
}

func (c resourceConfigCheckSessionContainerOwner) Find(conn DbConn) (sq.Eq, bool, error) {
	var ids []int
	rows, err := psql.Select("id").
		From("resource_config_check_sessions").
		Where(sq.And{
			sq.Eq{"resource_config_id": c.resourceConfigID},
			sq.Expr("expires_at > NOW()"),
		}).
		RunWith(conn).
		Query()
	if err != nil {
		return nil, false, err
	}

	for rows.Next() {
		var id int
		err = rows.Scan(&id)
		if err != nil {
			return nil, false, err
		}

		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return nil, false, nil
	}

	return sq.Eq{
		"resource_config_check_session_id": ids,
	}, true, nil
}

func (c resourceConfigCheckSessionContainerOwner) Create(tx Tx, workerName string) (map[string]interface{}, error) {
	var wbrtID int
	err := psql.Select("id").
		From("worker_base_resource_types").
		Where(sq.Eq{
			"worker_name":           workerName,
			"base_resource_type_id": c.baseResourceTypeID,
		}).
		Suffix("FOR SHARE").
		RunWith(tx).
		QueryRow().
		Scan(&wbrtID)
	if err != nil {
		return nil, fmt.Errorf("get worker base resource type id: %s", err)
	}

	expiryStmt := fmt.Sprintf(
		"NOW() + LEAST(GREATEST('%d seconds'::interval, NOW() - max(start_time)), '%d seconds'::interval)",
		int(c.expiries.Min.Seconds()),
		int(c.expiries.Max.Seconds()),
	)

	var rccsID int
	err = psql.Insert("resource_config_check_sessions").
		SetMap(map[string]interface{}{
			"resource_config_id":           c.resourceConfigID,
			"worker_base_resource_type_id": wbrtID,
			"expires_at":                   sq.Expr("(SELECT " + expiryStmt + " FROM workers)"),
		}).
		Suffix(`
			ON CONFLICT (resource_config_id, worker_base_resource_type_id) DO UPDATE SET
				resource_config_id = ?,
				worker_base_resource_type_id = ?
			RETURNING id
		`, c.resourceConfigID, wbrtID).
		RunWith(tx).
		QueryRow().
		Scan(&rccsID)
	if err != nil {
		return nil, fmt.Errorf("upsert resource config check session: %s", err)
	}

	return map[string]interface{}{
		"resource_config_check_session_id": rccsID,
	}, nil

}

// NewFixedHandleContainerOwner is used in testing to represent a container
// with a fixed handle, rather than using the randomly generated UUID as a
// handle.
func NewFixedHandleContainerOwner(handle string) ContainerOwner {
	return fixedHandleContainerOwner{
		Handle: handle,
	}
}

type fixedHandleContainerOwner struct {
	Handle string
}

func (c fixedHandleContainerOwner) Find(DbConn) (sq.Eq, bool, error) {
	return sq.Eq(c.sqlMap()), true, nil
}

func (c fixedHandleContainerOwner) Create(Tx, string) (map[string]interface{}, error) {
	return c.sqlMap(), nil
}

func (c fixedHandleContainerOwner) sqlMap() map[string]interface{} {
	return map[string]interface{}{
		"handle": c.Handle,
	}
}

// NewInMemoryCheckBuildContainerOwner references a in-memory check build. To
// reduce burden to db, this will use in-memory build's pre-id. And to ensure
// pre-id is unique, in-memory's create time is also used.
func NewInMemoryCheckBuildContainerOwner(
	buildID int,
	createTime time.Time,
	planID atc.PlanID,
	teamID int,
) ContainerOwner {
	return inMemoryCheckBuildContainerOwner{
		BuildID:    buildID,
		CreateTime: createTime,
		PlanID:     planID,
		TeamID:     teamID,
	}
}

type inMemoryCheckBuildContainerOwner struct {
	BuildID    int
	CreateTime time.Time
	PlanID     atc.PlanID
	TeamID     int
}

func (c inMemoryCheckBuildContainerOwner) Find(DbConn) (sq.Eq, bool, error) {
	return sq.Eq(c.sqlMap()), true, nil
}

func (c inMemoryCheckBuildContainerOwner) Create(Tx, string) (map[string]interface{}, error) {
	return c.sqlMap(), nil
}

func (c inMemoryCheckBuildContainerOwner) sqlMap() map[string]interface{} {
	return map[string]interface{}{
		"in_memory_build_id":          c.BuildID,
		"in_memory_build_create_time": c.CreateTime,
		"plan_id":                     c.PlanID,
		"team_id":                     c.TeamID,
	}
}

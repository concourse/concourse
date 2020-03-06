package db

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
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
	teamID int,
) ContainerOwner {
	return imageCheckContainerOwner{
		Container: container,
		TeamID:    teamID,
	}
}

type imageCheckContainerOwner struct {
	Container CreatingContainer
	TeamID    int
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
		"team_id":                  c.TeamID,
	}
}

// NewImageGetContainerOwner references a container whose image resource this
// container is fetching. When the referenced container transitions to another
// state, or disappears, the container can be removed.
func NewImageGetContainerOwner(
	container CreatingContainer,
	teamID int,
) ContainerOwner {
	return imageGetContainerOwner{
		Container: container,
		TeamID:    teamID,
	}
}

type imageGetContainerOwner struct {
	Container CreatingContainer
	TeamID    int
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
		"team_id":                c.TeamID,
	}
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
		"team_id":  c.TeamID,
	}
}

func NewCheckContainerOwner(
	checkID int,
) ContainerOwner {
	return checkContainerOwner{
		CheckID: checkID,
	}
}

type checkContainerOwner struct {
	CheckID int
}

func (c checkContainerOwner) Find(Conn) (sq.Eq, bool, error) {
	return sq.Eq(c.sqlMap()), true, nil
}

func (c checkContainerOwner) Create(Tx, string) (map[string]interface{}, error) {
	return c.sqlMap(), nil
}

func (c checkContainerOwner) sqlMap() map[string]interface{} {
	return map[string]interface{}{
		"check_id": c.CheckID,
	}
}

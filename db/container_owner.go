package db

import "github.com/concourse/atc"

//go:generate counterfeiter . ContainerOwner

// ContainerOwner designates the data the container should reference that
// identifies its lifecycle. When the owner goes away, the container should
// be garbage collected.
type ContainerOwner interface {
	SetMap() map[string]interface{}
}

type creatingContainerContainerOwner struct {
	Container CreatingContainer
}

// NewCreatingContainerContainerOwner references a container that is in
// creating state. When the referenced container transitions to another state,
// or disappears, the container can be removed.
//
// This is used for the 'check' and 'get' containers created when fetching an
// image for a container.
func NewCreatingContainerContainerOwner(
	container CreatingContainer,
) ContainerOwner {
	return creatingContainerContainerOwner{
		Container: container,
	}
}

func (c creatingContainerContainerOwner) SetMap() map[string]interface{} {
	return map[string]interface{}{
		"creating_container_id": c.Container.ID(),
	}
}

type buildStepContainerOwner struct {
	BuildID int
	PlanID  atc.PlanID
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

func (c buildStepContainerOwner) SetMap() map[string]interface{} {
	return map[string]interface{}{
		"build_id": c.BuildID,
		"plan_id":  c.PlanID,
	}
}

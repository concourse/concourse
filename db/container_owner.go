package db

import "github.com/concourse/atc"

//go:generate counterfeiter . ContainerOwner

// ContainerOwner designates the data the container should reference that
// identifies its lifecycle. When the owner goes away, the container should
// be garbage collected.
type ContainerOwner interface {
	SQLMap() map[string]interface{}
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

func (c imageCheckContainerOwner) SQLMap() map[string]interface{} {
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

func (c imageGetContainerOwner) SQLMap() map[string]interface{} {
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

func (c buildStepContainerOwner) SQLMap() map[string]interface{} {
	return map[string]interface{}{
		"build_id": c.BuildID,
		"plan_id":  c.PlanID,
	}
}

package db

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
// creating state. When the container transitions to another state, or
// disappears, the ownership is nullified.
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
		"creating_container_id":    c.Container.ID(),
		"creating_container_state": ContainerStateCreating,
	}
}

package present

import "github.com/concourse/atc/worker"

func Container(container worker.Container) PresentedContainer {
	identifier := container.IdentifierFromProperties()
	return PresentedContainer{
		ID:           container.Handle(),
		PipelineName: identifier.PipelineName,
		Type:         identifier.Type,
		Name:         identifier.Name,
		BuildID:      identifier.BuildID,
	}
}

type PresentedContainer struct {
	ID           string
	PipelineName string
	Type         worker.ContainerType
	Name         string
	BuildID      int
}

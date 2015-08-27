package present

import "github.com/concourse/atc/worker"

func Container(container worker.Container) PresentedContainer {
	properties := container.IdentifierFromProperties()
	return PresentedContainer{
		ID:           container.Handle(),
		PipelineName: properties.PipelineName,
		Type:         properties.Type,
		Name:         properties.Name,
		BuildID:      properties.BuildID,
	}
}

type PresentedContainer struct {
	ID           string
	PipelineName string
	Type         worker.ContainerType
	Name         string
	BuildID      int
}

package handles

import (
	"fmt"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/gc"
)

type containerSyncer struct {
	containerRepository db.ContainerRepository
	destroyer           gc.Destroyer
}

func NewContainerSyncer(
	containerRepository db.ContainerRepository,
	destroyer gc.Destroyer,
) *containerSyncer {
	return &containerSyncer{
		containerRepository: containerRepository,
		destroyer:           destroyer,
	}
}

func (r containerSyncer) Sync(handles []string, worker string) error {
	_, err := r.containerRepository.DestroyUnknownContainers(worker, handles)
	if err != nil {
		return fmt.Errorf("destroy unknown containers: %w", err)
	}

	err = r.containerRepository.UpdateContainersMissingSince(worker, handles)
	if err != nil {
		return fmt.Errorf("update containers missing since: %w", err)
	}

	err = r.destroyer.DestroyContainers(worker, handles)
	if err != nil {
		return fmt.Errorf("destroy containers: %w", err)
	}

	return nil
}

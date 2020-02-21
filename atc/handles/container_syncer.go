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

//
// handles: containers that exist in the worker
//
func (r containerSyncer) Sync(handles []string, worker string) error {
	// add to the `db` (as destroying) those containers that are not known
	// about in the db but exist in the worker so that they can later be
	// destroyed by the traditional gc path
	//
	_, err := r.containerRepository.DestroyUnknownContainers(worker, handles)
	if err != nil {
		return fmt.Errorf("destroy unknown containers: %w", err)
	}

	err = r.containerRepository.UpdateContainersMissingSince(worker, handles)
	if err != nil {
		return fmt.Errorf("update containers missing since: %w", err)
	}

	// remove all of the containers marked as "destroying" except for these
	// that we're supplying.
	//
	err = r.destroyer.DestroyContainers(worker, handles)
	if err != nil {
		return fmt.Errorf("destroy containers: %w", err)
	}

	return nil
}

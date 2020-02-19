package handles

import (
	"fmt"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/gc"
)

type volumeSyncer struct {
	repository db.VolumeRepository
	destroyer  gc.Destroyer
}

func NewVolumeSyncer(
	repository db.VolumeRepository,
	destroyer gc.Destroyer,
) *volumeSyncer {
	return &volumeSyncer{
		repository: repository,
		destroyer:  destroyer,
	}
}

func (r volumeSyncer) Sync(handles []string, worker string) error {
	_, err := r.repository.DestroyUnknownVolumes(worker, handles)
	if err != nil {
		return fmt.Errorf("destroy unknown volumes: %w", err)
	}

	err = r.repository.UpdateVolumesMissingSince(worker, handles)
	if err != nil {
		return fmt.Errorf("update volumes missing since: %w", err)
	}

	err = r.destroyer.DestroyVolumes(worker, handles)
	if err != nil {
		return fmt.Errorf("destroy volumes: %w", err)
	}

	return nil
}

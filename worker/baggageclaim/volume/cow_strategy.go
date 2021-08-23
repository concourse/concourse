package volume

import (
	"errors"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/worker/baggageclaim"
)

var ErrNoParentVolumeProvided = errors.New("no parent volume provided")
var ErrParentVolumeNotFound = errors.New("parent volume not found")

type COWStrategy struct {
	ParentHandle string
}

func (strategy COWStrategy) Materialize(logger lager.Logger, handle string, fs Filesystem, streamer Streamer) (FilesystemInitVolume, error) {
	if strategy.ParentHandle == "" {
		logger.Info("parent-not-specified")
		return nil, ErrNoParentVolumeProvided
	}

	parentVolume, found, err := fs.LookupVolume(strategy.ParentHandle)
	if err != nil {
		logger.Error("failed-to-lookup-parent", err)
		return nil, err
	}

	if !found {
		logger.Info("parent-not-found")
		return nil, ErrParentVolumeNotFound
	}

	return parentVolume.NewSubvolume(handle)
}

func (strateg COWStrategy) String() string {
	return baggageclaim.StrategyCopyOnWrite
}

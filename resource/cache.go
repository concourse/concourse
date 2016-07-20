package resource

import "github.com/concourse/atc/worker"

const initializedProperty = "initialized"

type noopCache struct{}

func (noopCache) IsInitialized() (bool, error) { return false, nil }
func (noopCache) Initialize() error            { return nil }
func (noopCache) Volume() worker.Volume        { return nil }

type volumeCache struct {
	volume worker.Volume
}

func (cache volumeCache) IsInitialized() (bool, error) {
	props, err := cache.volume.Properties()
	if err != nil {
		return false, err
	}

	_, found := props[initializedProperty]
	return found, nil
}

func (cache volumeCache) Initialize() error {
	return cache.volume.SetProperty("initialized", "yep")
}

func (cache volumeCache) Volume() worker.Volume {
	return cache.volume
}

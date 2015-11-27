package resource

import (
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/lager"
)

const resourceCacheTTL = 87600 * time.Hour
const reapExtraVolumeTTL = time.Minute

//go:generate counterfeiter . CacheIdentifier

type CacheIdentifier interface {
	FindOn(lager.Logger, baggageclaim.Client) (baggageclaim.Volume, bool, error)
	CreateOn(lager.Logger, baggageclaim.Client) (baggageclaim.Volume, error)

	ResourceVersion() atc.Version
	ResourceHash() string
}

type ResourceCacheIdentifier struct {
	Type    ResourceType
	Version atc.Version
	Source  atc.Source
	Params  atc.Params
}

func (identifier ResourceCacheIdentifier) FindOn(logger lager.Logger, vm baggageclaim.Client) (baggageclaim.Volume, bool, error) {
	volumes, err := vm.ListVolumes(logger, identifier.initializedVolumeProperties())
	if err != nil {
		return nil, false, err
	}

	switch len(volumes) {
	case 0:
		return nil, false, nil
	case 1:
		return volumes[0], true, nil
	default:
		return selectLowestAlphabeticalVolume(logger, volumes), true, nil
	}
}

func (identifier ResourceCacheIdentifier) CreateOn(logger lager.Logger, vm baggageclaim.Client) (baggageclaim.Volume, error) {
	return vm.CreateVolume(logger, baggageclaim.VolumeSpec{
		Properties: identifier.volumeProperties(),
		TTL:        resourceCacheTTL,
		Privileged: true,
	})
}

func (identifier ResourceCacheIdentifier) volumeProperties() baggageclaim.VolumeProperties {
	source, _ := json.Marshal(identifier.Source)

	version, _ := json.Marshal(identifier.Version)

	params, _ := json.Marshal(identifier.Params)

	return baggageclaim.VolumeProperties{
		"resource-type":    string(identifier.Type),
		"resource-version": string(version),
		"resource-source":  shastr(source),
		"resource-params":  shastr(params),
	}
}

func (identifier ResourceCacheIdentifier) initializedVolumeProperties() baggageclaim.VolumeProperties {
	props := identifier.volumeProperties()
	props["initialized"] = "yep"
	return props
}

func (identifier ResourceCacheIdentifier) ResourceVersion() atc.Version {
	return identifier.Version
}

func (identifier ResourceCacheIdentifier) ResourceHash() string {
	return GenerateResourceHash(identifier.Source, string(identifier.Type))
}

func GenerateResourceHash(source atc.Source, resourceType string) string {
	sourceJSON, _ := json.Marshal(source)
	return resourceType + string(sourceJSON)
}

func shastr(b []byte) string {
	return fmt.Sprintf("%x", sha512.Sum512(b))
}

func selectLowestAlphabeticalVolume(logger lager.Logger, volumes []baggageclaim.Volume) baggageclaim.Volume {
	var lowestVolume baggageclaim.Volume

	for _, v := range volumes {
		if lowestVolume == nil {
			lowestVolume = v
		} else if v.Handle() < lowestVolume.Handle() {
			lowestVolume = v
		}
	}

	for _, v := range volumes {
		if v != lowestVolume {
			v.Release(reapExtraVolumeTTL)
		}
	}

	return lowestVolume
}

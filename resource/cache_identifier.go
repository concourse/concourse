package resource

import (
	"crypto/sha512"
	"encoding/json"
	"fmt"

	"github.com/concourse/atc"
	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . CacheIdentifier

const resourceTTLInSeconds = 60 * 60 * 24
const reapExtraVolumeTTLInSeconds = 60

type CacheIdentifier interface {
	FindOn(lager.Logger, baggageclaim.Client) (baggageclaim.Volume, bool, error)
	CreateOn(lager.Logger, baggageclaim.Client) (baggageclaim.Volume, error)
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
		Properties:   identifier.volumeProperties(),
		TTLInSeconds: resourceTTLInSeconds,
		Privileged:   true,
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
			reapLog := logger.Session("reaping-extra-cache")

			reapLog.Debug("reap", lager.Data{"volume": v.Handle()})

			v.Release()

			// setting TTL here is best-effort; don't worry about failure
			err := v.SetTTL(reapExtraVolumeTTLInSeconds)
			if err != nil {
				reapLog.Info("failed-to-set-ttl", lager.Data{
					"error":  err.Error(),
					"volume": v.Handle(),
				})
			}
		}
	}

	return lowestVolume
}

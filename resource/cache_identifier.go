package resource

import (
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
)

const reapExtraVolumeTTL = time.Minute

//go:generate counterfeiter . CacheIdentifier

type CacheIdentifier interface {
	FindOn(lager.Logger, worker.Client) (worker.Volume, bool, error)
	CreateOn(lager.Logger, worker.Client) (worker.Volume, error)

	VolumeIdentifier() worker.VolumeIdentifier
}

type ResourceCacheIdentifier struct {
	Type    ResourceType
	Version atc.Version
	Source  atc.Source
	Params  atc.Params
}

func (identifier ResourceCacheIdentifier) FindOn(logger lager.Logger, workerClient worker.Client) (worker.Volume, bool, error) {
	volumes, err := workerClient.ListVolumes(logger, identifier.initializedVolumeProperties())
	if err != nil {
		logger.Error("failed-to-list-volumes", err)
		return nil, false, err
	}

	switch len(volumes) {
	case 0:
		logger.Debug("no-volumes-found")
		return nil, false, nil
	case 1:
		return volumes[0], true, nil
	default:
		return selectLowestAlphabeticalVolume(logger, volumes), true, nil
	}
}

func (identifier ResourceCacheIdentifier) CreateOn(logger lager.Logger, workerClient worker.Client) (worker.Volume, error) {
	ttl := time.Duration(0)

	if identifier.Version == nil {
		ttl = worker.VolumeTTL
	}

	return workerClient.CreateVolume(
		logger,
		worker.VolumeSpec{
			Strategy: worker.ResourceCacheStrategy{
				ResourceVersion: identifier.Version,
				ResourceHash:    GenerateResourceHash(identifier.Source, string(identifier.Type)),
			},
			Properties: identifier.volumeProperties(),
			Privileged: true,
			TTL:        ttl,
		},
		0,
	)
}

func (identifier ResourceCacheIdentifier) volumeProperties() worker.VolumeProperties {
	source, _ := json.Marshal(identifier.Source)

	version, _ := json.Marshal(identifier.Version)

	params, _ := json.Marshal(identifier.Params)

	return worker.VolumeProperties{
		"resource-type":    string(identifier.Type),
		"resource-version": string(version),
		"resource-source":  shastr(source),
		"resource-params":  shastr(params),
	}
}

func (identifier ResourceCacheIdentifier) initializedVolumeProperties() worker.VolumeProperties {
	props := identifier.volumeProperties()
	props["initialized"] = "yep"
	return props
}

func (identifier ResourceCacheIdentifier) VolumeIdentifier() worker.VolumeIdentifier {
	return worker.VolumeIdentifier{
		ResourceCache: &db.ResourceCacheIdentifier{
			ResourceVersion: identifier.Version,
			ResourceHash:    GenerateResourceHash(identifier.Source, string(identifier.Type)),
		},
	}
}

func GenerateResourceHash(source atc.Source, resourceType string) string {
	sourceJSON, _ := json.Marshal(source)
	return resourceType + string(sourceJSON)
}

func shastr(b []byte) string {
	return fmt.Sprintf("%x", sha512.Sum512(b))
}

func selectLowestAlphabeticalVolume(logger lager.Logger, volumes []worker.Volume) worker.Volume {
	var lowestVolume worker.Volume

	for _, v := range volumes {
		if lowestVolume == nil {
			lowestVolume = v
		} else if v.Handle() < lowestVolume.Handle() {
			lowestVolume = v
		}
	}

	for _, v := range volumes {
		if v != lowestVolume {
			v.Release(worker.FinalTTL(reapExtraVolumeTTL))
		}
	}

	return lowestVolume
}

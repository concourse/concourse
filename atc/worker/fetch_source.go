package worker

// this file takes in a resource and returns a source (Volume)
// we might not need to model this way

import (
	"context"
	"fmt"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/runtime"
)

//go:generate counterfeiter . FetchSource

type FetchSource interface {
	Find() (GetResult, Volume, bool, error)
	Create(context.Context) (GetResult, Volume, error)
}

//go:generate counterfeiter . FetchSourceFactory

type FetchSourceFactory interface {
	NewFetchSource(
		logger lager.Logger,
		worker Worker,
		owner db.ContainerOwner,
		cache db.UsedResourceCache,
		resource resource.Resource,
		containerSpec ContainerSpec,
		processSpec runtime.ProcessSpec,
		containerMetadata db.ContainerMetadata,
	) FetchSource
}

type fetchSourceFactory struct {
	resourceCacheFactory db.ResourceCacheFactory
}

func NewFetchSourceFactory(
	resourceCacheFactory db.ResourceCacheFactory,
) FetchSourceFactory {
	return &fetchSourceFactory{
		resourceCacheFactory: resourceCacheFactory,
	}
}

func (r *fetchSourceFactory) NewFetchSource(
	logger lager.Logger,
	worker Worker,
	owner db.ContainerOwner,
	cache db.UsedResourceCache,
	resource resource.Resource,
	containerSpec ContainerSpec,
	processSpec runtime.ProcessSpec,
	containerMetadata db.ContainerMetadata,
) FetchSource {
	return &fetchSource{
		logger:                 logger,
		worker:                 worker,
		owner:                  owner,
		cache:                  cache,
		resource:               resource,
		containerSpec:          containerSpec,
		processSpec:            processSpec,
		containerMetadata:      containerMetadata,
		dbResourceCacheFactory: r.resourceCacheFactory,
	}
}

type fetchSource struct {
	logger                 lager.Logger
	worker                 Worker
	owner                  db.ContainerOwner
	cache                  db.UsedResourceCache
	resource               resource.Resource
	containerSpec          ContainerSpec
	processSpec            runtime.ProcessSpec
	containerMetadata      db.ContainerMetadata
	dbResourceCacheFactory db.ResourceCacheFactory
}

func (s *fetchSource) Find() (GetResult, Volume, bool, error) {
	sLog := s.logger.Session("find")
	result := GetResult{}

	volume, found, err := s.worker.FindVolumeForResourceCache(s.logger, s.cache)
	if err != nil {
		sLog.Error("EVAN-failed-to-find-initialized-on", err)
		return result, nil, false, err
	}

	if !found {
		return result, nil, false, nil
	}

	metadata, err := s.dbResourceCacheFactory.ResourceCacheMetadata(s.cache)
	if err != nil {
		sLog.Error("failed-to-get-resource-cache-metadata", err)
		return result, nil, false, err
	}

	s.logger.Debug("found-initialized-versioned-source", lager.Data{
		"version":  s.cache.Version(),
		"metadata": metadata.ToATCMetadata(),
	})

	atcMetaData := []atc.MetadataField{}
	for _, m := range metadata {
		atcMetaData = append(atcMetaData, atc.MetadataField{
			Name:  m.Name,
			Value: m.Value,
		})
	}

	return GetResult{
			ExitStatus: 0,
			VersionResult: runtime.VersionResult{
				Version:  s.cache.Version(),
				Metadata: atcMetaData,
			},
			GetArtifact: runtime.GetArtifact{VolumeHandle: volume.Handle()},
		},
		volume, true, nil
}

// Create runs under the lock but we need to make sure volume does not exist
// yet before creating it under the lock
func (s *fetchSource) Create(ctx context.Context) (GetResult, Volume, error) {
	sLog := s.logger.Session("create")

	fmt.Fprintf(os.Stderr, "EVAN:fetchSource.Create, cache_id=%d\n", s.cache.ID())

	findResult, volume, found, err := s.Find()
	if err != nil {
		return GetResult{}, nil, err
	}

	if found {
		return findResult, volume, nil
	}

	s.containerSpec.BindMounts = []BindMountSource{
		&CertsVolumeMount{Logger: s.logger},
	}

	container, err := s.worker.FindOrCreateContainer(
		ctx,
		s.logger,
		s.owner,
		s.containerMetadata,
		s.containerSpec,
	)

	if err != nil {
		sLog.Error("failed-to-construct-resource", err)
		return GetResult{}, nil, err
	}

	vr, err := s.resource.Get(ctx, s.processSpec, container)
	if err != nil {
		sLog.Error("failed-to-fetch-resource", err)
		// TODO: Is this compatible with previous behaviour of returning a nil when error type is NOT ErrResourceScriptFailed

		if failErr, ok := err.(runtime.ErrResourceScriptFailed); ok {
			return GetResult{
				ExitStatus: failErr.ExitStatus,
			}, nil, nil
		}
		return GetResult{}, nil, err
	}

	volume = volumeWithFetchedBits(s.processSpec.Args[0], container)

	err = volume.SetPrivileged(false)
	if err != nil {
		sLog.Error("failed-to-set-volume-unprivileged", err)
		return GetResult{}, nil, err
	}

	err = volume.InitializeResourceCache(s.cache)
	if err != nil {
		sLog.Error("failed-to-initialize-cache", err)
		return GetResult{}, nil, err
	}

	err = s.dbResourceCacheFactory.UpdateResourceCacheMetadata(s.cache, vr.Metadata)
	if err != nil {
		s.logger.Error("failed-to-update-resource-cache-metadata", err, lager.Data{"resource-cache": s.cache})
		return GetResult{}, nil, err
	}

	return GetResult{
		ExitStatus:    0,
		VersionResult: vr,
		GetArtifact: runtime.GetArtifact{
			VolumeHandle: volume.Handle(),
		},
	}, volume, nil
}

func volumeWithFetchedBits(bitsDestinationPath string, container Container) Volume {
	for _, mount := range container.VolumeMounts() {
		if mount.MountPath == bitsDestinationPath {
			return mount.Volume
		}
	}
	return nil
}

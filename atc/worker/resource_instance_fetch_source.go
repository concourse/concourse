package worker

// this file takes in a resource and returns a source (Volume)
// we might not need to model this way

import (
	"context"

	"github.com/concourse/concourse/atc/resource"

	"github.com/concourse/concourse/atc/runtime"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter . FetchSource

type FetchSource interface {
	//LockName() (string, error)
	Find() (GetResult, Volume, bool, error)
	Create(context.Context) (GetResult, Volume, error)
}

//go:generate counterfeiter . FetchSourceFactory

type FetchSourceFactory interface {
	NewFetchSource(
		logger lager.Logger,
		worker Worker,
		owner db.ContainerOwner,
		resourceDir string,
		cache db.UsedResourceCache,
		resource resource.Resource,
		resourceTypes atc.VersionedResourceTypes,
		containerSpec ContainerSpec,
		processSpec runtime.ProcessSpec,
		containerMetadata db.ContainerMetadata,
		imageFetchingDelegate ImageFetchingDelegate,
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
	resourceDir string,
	cache db.UsedResourceCache,
	resource resource.Resource,
	resourceTypes atc.VersionedResourceTypes,
	containerSpec ContainerSpec,
	processSpec runtime.ProcessSpec,
	containerMetadata db.ContainerMetadata,
	imageFetchingDelegate ImageFetchingDelegate,
) FetchSource {
	return &resourceInstanceFetchSource{
		logger:                 logger,
		worker:                 worker,
		owner:                  owner,
		resourceDir:            resourceDir,
		cache:                  cache,
		resource:               resource,
		resourceTypes:          resourceTypes,
		containerSpec:          containerSpec,
		processSpec:            processSpec,
		containerMetadata:      containerMetadata,
		imageFetchingDelegate:  imageFetchingDelegate,
		dbResourceCacheFactory: r.resourceCacheFactory,
	}
}

type resourceInstanceFetchSource struct {
	logger                 lager.Logger
	worker                 Worker
	owner                  db.ContainerOwner
	resourceDir            string
	cache                  db.UsedResourceCache
	resource               resource.Resource
	resourceTypes          atc.VersionedResourceTypes
	containerSpec          ContainerSpec
	processSpec            runtime.ProcessSpec
	containerMetadata      db.ContainerMetadata
	imageFetchingDelegate  ImageFetchingDelegate
	dbResourceCacheFactory db.ResourceCacheFactory
}

func findOn(logger lager.Logger, w Worker, cache db.UsedResourceCache) (volume Volume, found bool, err error) {
	return w.FindVolumeForResourceCache(
		logger,
		cache,
	)
}

func (s *resourceInstanceFetchSource) Find() (GetResult, Volume, bool, error) {
	sLog := s.logger.Session("find")
	result := GetResult{}

	volume, found, err := findOn(s.logger, s.worker, s.cache)
	if err != nil {
		sLog.Error("failed-to-find-initialized-on", err)
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

	// TODO pass version down so it can be used in the log statement.
	//s.logger.Debug("found-initialized-versioned-source", lager.Data{"version": s.resourceInstance.Version(), "metadata": metadata.ToATCMetadata()})

	atcMetaData := []atc.MetadataField{}
	for _, m := range metadata {
		atcMetaData = append(atcMetaData, atc.MetadataField{
			Name:  m.Name,
			Value: m.Value,
		})
	}

	return GetResult{
			0,
			// todo: figure out what logically should be returned for VersionResult
			runtime.VersionResult{
				Metadata: atcMetaData,
			},
			runtime.GetArtifact{VolumeHandle: volume.Handle()},
			nil,
		},
		volume, true, nil
}

// Create runs under the lock but we need to make sure volume does not exist
// yet before creating it under the lock
func (s *resourceInstanceFetchSource) Create(ctx context.Context) (GetResult, Volume, error) {
	sLog := s.logger.Session("create")
	result := GetResult{}
	var volume Volume

	findResult, volume, found, err := s.Find()
	if err != nil {
		return result, nil, err
	}

	if found {
		return findResult, nil, nil
	}

	s.containerSpec.BindMounts = []BindMountSource{
		&CertsVolumeMount{Logger: s.logger},
	}

	container, err := s.worker.FindOrCreateContainer(
		ctx,
		s.logger,
		s.imageFetchingDelegate,
		s.owner,
		s.containerMetadata,
		s.containerSpec,
		s.resourceTypes,
	)

	if err != nil {
		sLog.Error("failed-to-construct-resource", err)
		result = GetResult{
			1,
			// todo: figure out what logically should be returned for VersionResult
			runtime.VersionResult{},
			runtime.GetArtifact{},
			err,
		}
		return result, volume, err
	}

	mountPath := s.resourceDir
	for _, mount := range container.VolumeMounts() {
		if mount.MountPath == mountPath {
			volume = mount.Volume
			break
		}
	}

	vr := runtime.VersionResult{}
	// TODO This is pure EVIL
	//events := make(chan runtime.Event, 100)

	vr, err = s.resource.Get(ctx, s.processSpec, container)

	if err != nil {
		sLog.Error("failed-to-fetch-resource", err)
		// TODO Is this compatible with previous behaviour of returning a nil when error type is NOT ErrResourceScriptFailed

		// if error returned from running the actual script
		if failErr, ok := err.(runtime.ErrResourceScriptFailed); ok {
			result = GetResult{failErr.ExitStatus, runtime.VersionResult{}, runtime.GetArtifact{}, failErr}
			return result, volume, nil
		}
		return result, volume, err
	}

	err = volume.SetPrivileged(false)
	if err != nil {
		sLog.Error("failed-to-set-volume-unprivileged", err)
		return result, nil, err
	}

	// this initializes the worker resource cache, not the actual core resource cache
	err = volume.InitializeResourceCache(s.cache)
	if err != nil {
		sLog.Error("failed-to-initialize-cache", err)
		return result, nil, err
	}

	err = s.dbResourceCacheFactory.UpdateResourceCacheMetadata(s.cache, vr.Metadata)
	if err != nil {
		s.logger.Error("failed-to-update-resource-cache-metadata", err, lager.Data{"resource-cache": s.cache})
		return result, nil, err
	}

	return GetResult{
		VersionResult: vr,
		GetArtifact: runtime.GetArtifact{
			VolumeHandle: volume.Handle(),
		},
	}, volume, nil
}

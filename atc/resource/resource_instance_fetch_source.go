package resource

import (
	"context"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	v2 "github.com/concourse/concourse/atc/resource/v2"
	"github.com/concourse/concourse/atc/worker"
)

//go:generate counterfeiter . FetchSource

type FetchSource interface {
	LockName() (string, error)
	Find() (worker.Volume, bool, error)
	Create(context.Context) (worker.Volume, error)
}

//go:generate counterfeiter . FetchSourceFactory

type FetchSourceFactory interface {
	NewFetchSource(
		logger lager.Logger,
		getEventHandler v2.GetEventHandler,
		worker worker.Worker,
		resourceInstance ResourceInstance,
		resourceTypes creds.VersionedResourceTypes,
		containerSpec worker.ContainerSpec,
		session Session,
		imageFetchingDelegate worker.ImageFetchingDelegate,
	) FetchSource
}

type fetchSourceFactory struct {
	resourceCacheFactory db.ResourceCacheFactory
	resourceFactory      ResourceFactory
}

func NewFetchSourceFactory(
	resourceCacheFactory db.ResourceCacheFactory,
	resourceFactory ResourceFactory,
) FetchSourceFactory {
	return &fetchSourceFactory{
		resourceCacheFactory: resourceCacheFactory,
		resourceFactory:      resourceFactory,
	}
}

func (r *fetchSourceFactory) NewFetchSource(
	logger lager.Logger,
	getEventHandler v2.GetEventHandler,
	worker worker.Worker,
	resourceInstance ResourceInstance,
	resourceTypes creds.VersionedResourceTypes,
	containerSpec worker.ContainerSpec,
	session Session,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) FetchSource {
	return &resourceInstanceFetchSource{
		logger:                 logger,
		getEventHandler:        getEventHandler,
		worker:                 worker,
		resourceInstance:       resourceInstance,
		resourceTypes:          resourceTypes,
		containerSpec:          containerSpec,
		session:                session,
		imageFetchingDelegate:  imageFetchingDelegate,
		dbResourceCacheFactory: r.resourceCacheFactory,
		resourceFactory:        r.resourceFactory,
	}
}

type resourceInstanceFetchSource struct {
	logger                 lager.Logger
	getEventHandler        v2.GetEventHandler
	worker                 worker.Worker
	resourceInstance       ResourceInstance
	resourceTypes          creds.VersionedResourceTypes
	containerSpec          worker.ContainerSpec
	session                Session
	imageFetchingDelegate  worker.ImageFetchingDelegate
	dbResourceCacheFactory db.ResourceCacheFactory
	resourceFactory        ResourceFactory
}

func (s *resourceInstanceFetchSource) LockName() (string, error) {
	return s.resourceInstance.LockName(s.worker.Name())
}

func (s *resourceInstanceFetchSource) Find() (worker.Volume, bool, error) {
	sLog := s.logger.Session("find")

	volume, found, err := s.resourceInstance.FindOn(s.logger, s.worker)
	if err != nil {
		sLog.Error("failed-to-find-initialized-on", err)
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	s.logger.Debug("found-initialized-versioned-source", lager.Data{"version": s.resourceInstance.Version()})

	return volume, true, nil
}

// Create runs under the lock but we need to make sure volume does not exist
// yet before creating it under the lock
func (s *resourceInstanceFetchSource) Create(ctx context.Context) (worker.Volume, error) {
	sLog := s.logger.Session("create")

	foundVolume, found, err := s.Find()
	if err != nil {
		return nil, err
	}

	if found {
		return foundVolume, nil
	}

	s.containerSpec.BindMounts = []worker.BindMountSource{
		&worker.CertsVolumeMount{Logger: s.logger},
	}

	container, err := s.worker.FindOrCreateContainer(
		ctx,
		s.logger,
		s.imageFetchingDelegate,
		s.resourceInstance.ContainerOwner(),
		s.session.Metadata,
		s.containerSpec,
		s.resourceTypes,
	)
	if err != nil {
		return nil, err
	}

	if err != nil {
		sLog.Error("failed-to-construct-resource", err)
		return nil, err
	}

	mountPath := atc.ResourcesDir("get")
	var volume worker.Volume
	for _, mount := range container.VolumeMounts() {
		if mount.MountPath == mountPath {
			volume = mount.Volume
			break
		}
	}

	resource, err := s.resourceFactory.NewResourceForContainer(ctx, container)
	if err != nil {
		return nil, err
	}

	err = resource.Get(
		ctx,
		s.getEventHandler,
		volume,
		atc.IOConfig{
			Stdout: s.imageFetchingDelegate.Stdout(),
			Stderr: s.imageFetchingDelegate.Stderr(),
		},
		s.resourceInstance.Source(),
		s.resourceInstance.Params(),
		s.resourceInstance.Space(),
		s.resourceInstance.Version(),
	)
	if err != nil {
		sLog.Error("failed-to-fetch-resource", err)
		return nil, err
	}

	err = volume.SetPrivileged(false)
	if err != nil {
		sLog.Error("failed-to-set-volume-unprivileged", err)
		return nil, err
	}

	err = volume.InitializeResourceCache(s.resourceInstance.ResourceCache())
	if err != nil {
		sLog.Error("failed-to-initialize-cache", err)
		return nil, err
	}

	return volume, nil
}

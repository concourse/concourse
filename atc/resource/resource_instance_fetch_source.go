package resource

import (
	"context"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/worker"
)

//go:generate counterfeiter . FetchSource

type FetchSource interface {
	LockName() (string, error)
	Find() (VersionedSource, bool, error)
	Create(context.Context) (VersionedSource, error)
}

type resourceInstanceFetchSource struct {
	logger                 lager.Logger
	worker                 worker.Worker
	resourceInstance       ResourceInstance
	resourceTypes          creds.VersionedResourceTypes
	containerSpec          worker.ContainerSpec
	session                Session
	imageFetchingDelegate  worker.ImageFetchingDelegate
	dbResourceCacheFactory db.ResourceCacheFactory
}

func NewResourceInstanceFetchSource(
	logger lager.Logger,
	worker worker.Worker,
	resourceInstance ResourceInstance,
	resourceTypes creds.VersionedResourceTypes,
	containerSpec worker.ContainerSpec,
	session Session,
	imageFetchingDelegate worker.ImageFetchingDelegate,
	dbResourceCacheFactory db.ResourceCacheFactory,
) FetchSource {
	return &resourceInstanceFetchSource{
		logger:                 logger,
		worker:                 worker,
		resourceInstance:       resourceInstance,
		resourceTypes:          resourceTypes,
		containerSpec:          containerSpec,
		session:                session,
		imageFetchingDelegate:  imageFetchingDelegate,
		dbResourceCacheFactory: dbResourceCacheFactory,
	}
}

func (s *resourceInstanceFetchSource) LockName() (string, error) {
	return s.resourceInstance.LockName(s.worker.Name())
}

func (s *resourceInstanceFetchSource) Find() (VersionedSource, bool, error) {
	sLog := s.logger.Session("find")

	volume, found, err := s.resourceInstance.FindOn(s.logger, s.worker)
	if err != nil {
		sLog.Error("failed-to-find-initialized-on", err)
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	metadata, err := s.dbResourceCacheFactory.ResourceCacheMetadata(s.resourceInstance.ResourceCache())
	if err != nil {
		sLog.Error("failed-to-get-resource-cache-metadata", err)
		return nil, false, err
	}

	s.logger.Debug("found-initialized-versioned-source", lager.Data{"version": s.resourceInstance.Version(), "metadata": metadata.ToATCMetadata()})

	return NewGetVersionedSource(
		volume,
		s.resourceInstance.Version(),
		metadata.ToATCMetadata(),
	), true, nil
}

// Create runs under the lock but we need to make sure volume does not exist
// yet before creating it under the lock
func (s *resourceInstanceFetchSource) Create(ctx context.Context) (VersionedSource, error) {
	sLog := s.logger.Session("create")

	versionedSource, found, err := s.Find()
	if err != nil {
		return nil, err
	}

	if found {
		return versionedSource, nil
	}

	resourceFactory := NewResourceFactory(s.worker)
	resource, err := resourceFactory.NewResource(
		ctx,
		s.logger,
		s.resourceInstance.ContainerOwner(),
		s.session.Metadata,
		s.containerSpec,
		s.resourceTypes,
		s.imageFetchingDelegate,
	)
	if err != nil {
		sLog.Error("failed-to-construct-resource", err)
		return nil, err
	}

	mountPath := ResourcesDir("get")
	var volume worker.Volume
	for _, mount := range resource.Container().VolumeMounts() {
		if mount.MountPath == mountPath {
			volume = mount.Volume
			break
		}
	}

	versionedSource, err = resource.Get(
		ctx,
		volume,
		IOConfig{
			Stdout: s.imageFetchingDelegate.Stdout(),
			Stderr: s.imageFetchingDelegate.Stderr(),
		},
		s.resourceInstance.Source(),
		s.resourceInstance.Params(),
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

	err = s.dbResourceCacheFactory.UpdateResourceCacheMetadata(s.resourceInstance.ResourceCache(), versionedSource.Metadata())
	if err != nil {
		s.logger.Error("failed-to-update-resource-cache-metadata", err, lager.Data{"resource-cache": s.resourceInstance.ResourceCache()})
		return nil, err
	}

	return versionedSource, nil
}

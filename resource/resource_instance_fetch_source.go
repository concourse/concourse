package resource

import (
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
)

type resourceInstanceFetchSource struct {
	logger                 lager.Logger
	resourceInstance       ResourceInstance
	worker                 worker.Worker
	resourceTypes          creds.VersionedResourceTypes
	tags                   atc.Tags
	teamID                 int
	session                Session
	metadata               Metadata
	imageFetchingDelegate  worker.ImageFetchingDelegate
	dbResourceCacheFactory db.ResourceCacheFactory
}

func NewResourceInstanceFetchSource(
	logger lager.Logger,
	resourceInstance ResourceInstance,
	worker worker.Worker,
	resourceTypes creds.VersionedResourceTypes,
	tags atc.Tags,
	teamID int,
	session Session,
	metadata Metadata,
	imageFetchingDelegate worker.ImageFetchingDelegate,
	dbResourceCacheFactory db.ResourceCacheFactory,
) FetchSource {
	return &resourceInstanceFetchSource{
		logger:                 logger,
		resourceInstance:       resourceInstance,
		worker:                 worker,
		resourceTypes:          resourceTypes,
		tags:                   tags,
		teamID:                 teamID,
		session:                session,
		metadata:               metadata,
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
func (s *resourceInstanceFetchSource) Create(signals <-chan os.Signal, ready chan<- struct{}) (VersionedSource, error) {
	sLog := s.logger.Session("create")

	versionedSource, found, err := s.Find()
	if err != nil {
		return nil, err
	}

	if found {
		return versionedSource, nil
	}

	mountPath := ResourcesDir("get")

	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: string(s.resourceInstance.ResourceType()),
		},
		Tags:   s.tags,
		TeamID: s.teamID,
		Env:    s.metadata.Env(),

		Outputs: map[string]string{
			"resource": mountPath,
		},
	}

	container, err := s.worker.FindOrCreateContainer(
		s.logger,
		nil,
		s.imageFetchingDelegate,
		s.resourceInstance.ContainerOwner(),
		s.session.Metadata,
		containerSpec,
		s.resourceTypes,
	)
	if err != nil {
		sLog.Error("failed-to-create-container", err)
		return nil, err
	}

	var volume worker.Volume
	for _, mount := range container.VolumeMounts() {
		if mount.MountPath == mountPath {
			volume = mount.Volume
			break
		}
	}

	versionedSource, err = NewResourceForContainer(container).Get(
		volume,
		IOConfig{
			Stdout: s.imageFetchingDelegate.Stdout(),
			Stderr: s.imageFetchingDelegate.Stderr(),
		},
		s.resourceInstance.Source(),
		s.resourceInstance.Params(),
		s.resourceInstance.Version(),
		signals,
		ready,
	)
	if err != nil {
		if err == ErrAborted {
			sLog.Error("get-run-resource-aborted", err, lager.Data{"container": container.Handle()})
			return nil, ErrInterrupted
		}

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

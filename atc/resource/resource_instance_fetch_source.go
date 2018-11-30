package resource

import (
	"context"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/worker"
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

	mountPath := atc.ResourcesDir("get")

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

	resourceFactory := NewResourceFactory(s.worker)
	resource, err := resourceFactory.NewResource(
		ctx,
		s.logger,
		s.resourceInstance.ContainerOwner(),
		s.session.Metadata,
		containerSpec,
		s.resourceTypes,
		s.imageFetchingDelegate,
		s.resourceInstance.ResourceCache().ResourceConfig(),
	)
	if err != nil {
		sLog.Error("failed-to-construct-resource", err)
		return nil, err
	}

	var volume worker.Volume
	for _, mount := range resource.Container().VolumeMounts() {
		if mount.MountPath == mountPath {
			volume = mount.Volume
			break
		}
	}

	err = resource.Get(
		ctx,
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

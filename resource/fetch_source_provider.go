package resource

import (
	"context"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
)

//go:generate counterfeiter . FetchSourceProviderFactory

type FetchSourceProviderFactory interface {
	NewFetchSourceProvider(
		logger lager.Logger,
		session Session,
		metadata Metadata,
		tags atc.Tags,
		teamID int,
		resourceTypes creds.VersionedResourceTypes,
		resourceInstance ResourceInstance,
		imageFetchingDelegate worker.ImageFetchingDelegate,
	) FetchSourceProvider
}

//go:generate counterfeiter . FetchSourceProvider

type FetchSourceProvider interface {
	Get() (FetchSource, error)
}

//go:generate counterfeiter . FetchSource

type FetchSource interface {
	LockName() (string, error)
	Find() (VersionedSource, bool, error)
	Create(context.Context) (VersionedSource, error)
}

type fetchSourceProviderFactory struct {
	workerClient           worker.Client
	dbResourceCacheFactory db.ResourceCacheFactory
}

func NewFetchSourceProviderFactory(
	workerClient worker.Client,
	dbResourceCacheFactory db.ResourceCacheFactory,
) FetchSourceProviderFactory {
	return &fetchSourceProviderFactory{
		workerClient:           workerClient,
		dbResourceCacheFactory: dbResourceCacheFactory,
	}
}

func (f *fetchSourceProviderFactory) NewFetchSourceProvider(
	logger lager.Logger,
	session Session,
	metadata Metadata,
	tags atc.Tags,
	teamID int,
	resourceTypes creds.VersionedResourceTypes,
	resourceInstance ResourceInstance,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) FetchSourceProvider {
	return &fetchSourceProvider{
		logger:                 logger,
		session:                session,
		metadata:               metadata,
		tags:                   tags,
		teamID:                 teamID,
		resourceTypes:          resourceTypes,
		resourceInstance:       resourceInstance,
		imageFetchingDelegate:  imageFetchingDelegate,
		workerClient:           f.workerClient,
		dbResourceCacheFactory: f.dbResourceCacheFactory,
	}
}

type fetchSourceProvider struct {
	logger                 lager.Logger
	session                Session
	metadata               Metadata
	tags                   atc.Tags
	teamID                 int
	resourceTypes          creds.VersionedResourceTypes
	resourceInstance       ResourceInstance
	workerClient           worker.Client
	imageFetchingDelegate  worker.ImageFetchingDelegate
	dbResourceCacheFactory db.ResourceCacheFactory
}

func (f *fetchSourceProvider) Get() (FetchSource, error) {
	resourceSpec := worker.WorkerSpec{
		ResourceType: string(f.resourceInstance.ResourceType()),
		Tags:         f.tags,
		TeamID:       f.teamID,
	}

	chosenWorker, err := f.workerClient.Satisfying(f.logger.Session("fetch-source-provider"), resourceSpec, f.resourceTypes)
	if err != nil {
		f.logger.Error("no-workers-satisfying-spec", err)
		return nil, err
	}

	return NewResourceInstanceFetchSource(
		f.logger,
		f.resourceInstance,
		chosenWorker,
		f.resourceTypes,
		f.tags,
		f.teamID,
		f.session,
		f.metadata,
		f.imageFetchingDelegate,
		f.dbResourceCacheFactory,
	), nil
}

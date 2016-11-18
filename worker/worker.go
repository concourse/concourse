package worker

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/dbng"
)

var ErrUnsupportedResourceType = errors.New("unsupported resource type")
var ErrIncompatiblePlatform = errors.New("incompatible platform")
var ErrMismatchedTags = errors.New("mismatched tags")
var ErrNoVolumeManager = errors.New("worker does not support volume management")
var ErrTeamMismatch = errors.New("mismatched team")
var ErrResourceTypeNotFound = errors.New("resource type not found")
var ErrNotImplemented = errors.New("Not implemented")

type MalformedMetadataError struct {
	UnmarshalError error
}

func (err MalformedMetadataError) Error() string {
	return fmt.Sprintf("malformed image metadata: %s", err.UnmarshalError)
}

const containerKeepalive = 30 * time.Second
const ContainerTTL = 5 * time.Minute

const ephemeralPropertyName = "concourse:ephemeral"
const volumePropertyName = "concourse:volumes"
const volumeMountsPropertyName = "concourse:volume-mounts"
const userPropertyName = "user"
const RawRootFSScheme = "raw"
const ImageMetadataFile = "metadata.json"

//go:generate counterfeiter . Worker

type Worker interface {
	Client

	ActiveContainers() int

	Description() string
	Name() string
	ResourceTypes() []atc.WorkerResourceType
	Tags() atc.Tags
	Uptime() time.Duration
	IsOwnedByTeam() bool
}

//go:generate counterfeiter . DBContainerFactory

type DBContainerFactory interface {
	CreateBuildContainer(
		worker *dbng.Worker,
		build *dbng.Build,
		planID atc.PlanID,
		meta dbng.ContainerMetadata,
	) (*dbng.CreatingContainer, error)

	CreateResourceGetContainer(
		worker *dbng.Worker,
		resourceCache *dbng.UsedResourceCache,
		stepName string,
	) (*dbng.CreatingContainer, error)

	CreateResourceCheckContainer(
		worker *dbng.Worker,
		resourceConfig *dbng.UsedResourceConfig,
		stepName string,
	) (*dbng.CreatingContainer, error)

	FindContainer(handle string) (*dbng.CreatedContainer, bool, error)
	ContainerCreated(*dbng.CreatingContainer, string) (*dbng.CreatedContainer, error)
}

//go:generate counterfeiter . GardenWorkerDB

type GardenWorkerDB interface {
	CreateContainer(container db.Container, ttl time.Duration, maxLifetime time.Duration, volumeHandles []string) (db.SavedContainer, error)
	UpdateContainerTTLToBeRemoved(container db.Container, ttl time.Duration, maxLifetime time.Duration) (db.SavedContainer, error)
	GetContainer(handle string) (db.SavedContainer, bool, error)
	UpdateExpiresAtOnContainer(handle string, ttl time.Duration) error
	ReapContainer(string) error
	GetPipelineByID(pipelineID int) (db.SavedPipeline, error)
	AcquireVolumeCreatingLock(lager.Logger, int) (lock.Lock, bool, error)
}

type gardenWorker struct {
	containerProviderFactory ContainerProviderFactory

	volumeClient            VolumeClient
	pipelineDBFactory       db.PipelineDBFactory
	dbContainerFactory      DBContainerFactory
	dbResourceCacheFactory  dbng.ResourceCacheFactory
	dbResourceConfigFactory dbng.ResourceConfigFactory

	db       GardenWorkerDB
	provider WorkerProvider

	clock clock.Clock

	activeContainers int
	resourceTypes    []atc.WorkerResourceType
	platform         string
	tags             atc.Tags
	teamID           int
	name             string
	addr             string
	startTime        int64
	httpProxyURL     string
	httpsProxyURL    string
	noProxy          string
}

func NewGardenWorker(
	containerProviderFactory ContainerProviderFactory,
	volumeClient VolumeClient,
	pipelineDBFactory db.PipelineDBFactory,
	dbContainerFactory DBContainerFactory,
	dbResourceCacheFactory dbng.ResourceCacheFactory,
	dbResourceConfigFactory dbng.ResourceConfigFactory,
	db GardenWorkerDB,
	provider WorkerProvider,
	clock clock.Clock,
	activeContainers int,
	resourceTypes []atc.WorkerResourceType,
	platform string,
	tags atc.Tags,
	teamID int,
	name string,
	addr string,
	startTime int64,
	httpProxyURL string,
	httpsProxyURL string,
	noProxy string,
) Worker {
	return &gardenWorker{
		containerProviderFactory: containerProviderFactory,

		volumeClient:            volumeClient,
		dbContainerFactory:      dbContainerFactory,
		dbResourceCacheFactory:  dbResourceCacheFactory,
		dbResourceConfigFactory: dbResourceConfigFactory,

		db:                db,
		provider:          provider,
		clock:             clock,
		pipelineDBFactory: pipelineDBFactory,
		activeContainers:  activeContainers,
		resourceTypes:     resourceTypes,
		platform:          platform,
		tags:              tags,
		teamID:            teamID,
		name:              name,
		addr:              addr,
		startTime:         startTime,
		httpProxyURL:      httpProxyURL,
		httpsProxyURL:     httpsProxyURL,
		noProxy:           noProxy,
	}
}

func (worker *gardenWorker) FindResourceTypeByPath(path string) (atc.WorkerResourceType, bool) {
	for _, rt := range worker.resourceTypes {
		if path == rt.Image {
			return rt, true
		}
	}

	return atc.WorkerResourceType{}, false
}

func (worker *gardenWorker) FindOrCreateVolumeForResourceCache(logger lager.Logger, volumeSpec VolumeSpec, resourceCache *dbng.UsedResourceCache) (Volume, error) {
	return worker.volumeClient.FindOrCreateVolumeForResourceCache(logger, volumeSpec, resourceCache)
}

func (worker *gardenWorker) FindInitializedVolumeForResourceCache(logger lager.Logger, resourceCache *dbng.UsedResourceCache) (Volume, bool, error) {
	return worker.volumeClient.FindInitializedVolumeForResourceCache(logger, resourceCache)
}

func (worker *gardenWorker) LookupVolume(logger lager.Logger, handle string) (Volume, bool, error) {
	return worker.volumeClient.LookupVolume(logger, handle)
}

func (worker *gardenWorker) ValidateResourceCheckVersion(container db.SavedContainer) (bool, error) {
	if container.Type != db.ContainerTypeCheck || container.CheckType == "" || container.ResourceTypeVersion == nil {
		return true, nil
	}

	if container.PipelineID > 0 {
		savedPipeline, err := worker.db.GetPipelineByID(container.PipelineID)
		if err != nil {
			return false, err
		}

		pipelineDB := worker.pipelineDBFactory.Build(savedPipeline)

		_, found, err := pipelineDB.GetResourceType(container.CheckType)
		if err != nil {
			return false, err
		}

		// this is custom resource type, do not validate version on worker
		if found {
			return true, nil
		}
	}

	for _, workerResourceType := range worker.resourceTypes {
		if container.CheckType == workerResourceType.Type && workerResourceType.Version == container.ResourceTypeVersion[container.CheckType] {
			return true, nil
		}
	}

	return false, nil
}

func (worker *gardenWorker) CreateBuildContainer(
	logger lager.Logger,
	cancel <-chan os.Signal,
	delegate ImageFetchingDelegate,
	id Identifier,
	metadata Metadata,
	spec ContainerSpec,
	resourceTypes atc.ResourceTypes,
	outputPaths map[string]string,
) (Container, error) {
	creatingContainer, err := worker.dbContainerFactory.CreateBuildContainer(
		&dbng.Worker{
			Name:       worker.name,
			GardenAddr: &worker.addr,
		},
		&dbng.Build{
			ID: id.BuildID,
		},
		id.PlanID,
		dbng.ContainerMetadata{
			Name: metadata.StepName,
			Type: string(metadata.Type),
		},
	)

	if err != nil {
		return nil, err
	}

	containerProvider := worker.containerProviderFactory.ContainerProviderFor(worker)
	return containerProvider.FindOrCreateContainer(
		logger,
		cancel,
		creatingContainer,
		delegate,
		id,
		metadata,
		spec,
		resourceTypes,
		outputPaths,
	)
}

func (worker *gardenWorker) FindOrCreateResourceGetContainer(
	logger lager.Logger,
	cancel <-chan os.Signal,
	delegate ImageFetchingDelegate,
	id Identifier,
	metadata Metadata,
	spec ContainerSpec,
	resourceTypes atc.ResourceTypes,
	outputPaths map[string]string,
	resourceTypeName string,
	version atc.Version,
	source atc.Source,
	params atc.Params,
) (Container, error) {
	var resourceCache *dbng.UsedResourceCache

	if id.BuildID != 0 {
		var err error
		resourceCache, err = worker.dbResourceCacheFactory.FindOrCreateResourceCacheForBuild(
			logger,
			&dbng.Build{ID: id.BuildID},
			resourceTypeName,
			version,
			source,
			params,
			&dbng.Pipeline{ID: metadata.PipelineID},
			resourceTypes,
		)
		if err != nil {
			logger.Error("failed-to-get-resource-cache-for-build", err, lager.Data{"build-id": id.BuildID})
			return nil, err
		}
	} else if id.ResourceID != 0 {
		var err error
		resourceCache, err = worker.dbResourceCacheFactory.FindOrCreateResourceCacheForResource(
			logger,
			&dbng.Resource{
				ID: id.ResourceID,
			},
			resourceTypeName,
			version,
			source,
			params,
			&dbng.Pipeline{ID: metadata.PipelineID},
			resourceTypes,
		)
		if err != nil {
			logger.Error("failed-to-get-resource-cache-for-resource", err, lager.Data{"resource-id": id.ResourceID})
			return nil, err
		}
	} else {
		var err error
		resourceCache, err = worker.dbResourceCacheFactory.FindOrCreateResourceCacheForResourceType(
			logger,
			resourceTypeName,
			version,
			source,
			params,
			&dbng.Pipeline{ID: metadata.PipelineID},
			resourceTypes,
		)
		if err != nil {
			logger.Error("failed-to-get-resource-cache-for-resource-type", err, lager.Data{"resource-type": resourceTypeName})
			return nil, err
		}
	}

	creatingContainer, err := worker.dbContainerFactory.CreateResourceGetContainer(
		&dbng.Worker{
			Name:       worker.name,
			GardenAddr: &worker.addr,
		},
		resourceCache,
		metadata.StepName,
	)

	if err != nil {
		return nil, err
	}

	containerProvider := worker.containerProviderFactory.ContainerProviderFor(worker)
	return containerProvider.FindOrCreateContainer(
		logger,
		cancel,
		creatingContainer,
		delegate,
		id,
		metadata,
		spec,
		resourceTypes,
		outputPaths,
	)
}

func (worker *gardenWorker) CreateResourceCheckContainer(
	logger lager.Logger,
	cancel <-chan os.Signal,
	delegate ImageFetchingDelegate,
	id Identifier,
	metadata Metadata,
	spec ContainerSpec,
	resourceTypes atc.ResourceTypes,
	resourceType string,
	source atc.Source,
) (Container, error) {
	resourceConfig, err := worker.dbResourceConfigFactory.FindOrCreateResourceConfigForResource(
		logger,
		&dbng.Resource{
			ID: id.ResourceID,
		},
		resourceType,
		source,
		&dbng.Pipeline{ID: metadata.PipelineID},
		resourceTypes,
	)
	if err != nil {
		logger.Error("failed-to-get-resource-config", err)
		return nil, err
	}

	creatingContainer, err := worker.dbContainerFactory.CreateResourceCheckContainer(
		&dbng.Worker{
			Name:       worker.name,
			GardenAddr: &worker.addr,
		},
		resourceConfig,
		metadata.StepName,
	)
	if err != nil {
		logger.Error("failed-to-create-check-container", err)
		return nil, err
	}

	containerProvider := worker.containerProviderFactory.ContainerProviderFor(worker)
	return containerProvider.FindOrCreateContainer(logger, cancel, creatingContainer, delegate, id, metadata, spec, resourceTypes, map[string]string{})
}

func (worker *gardenWorker) CreateResourceTypeCheckContainer(
	logger lager.Logger,
	cancel <-chan os.Signal,
	delegate ImageFetchingDelegate,
	id Identifier,
	metadata Metadata,
	spec ContainerSpec,
	resourceTypes atc.ResourceTypes,
	resourceTypeName string,
	source atc.Source,
) (Container, error) {
	resourceConfig, err := worker.dbResourceConfigFactory.FindOrCreateResourceConfigForResourceType(
		logger,
		resourceTypeName,
		source,
		&dbng.Pipeline{ID: metadata.PipelineID},
		resourceTypes,
	)
	if err != nil {
		return nil, err
	}

	creatingContainer, err := worker.dbContainerFactory.CreateResourceCheckContainer(
		&dbng.Worker{
			Name:       worker.name,
			GardenAddr: &worker.addr,
		},
		resourceConfig,
		metadata.StepName,
	)
	if err != nil {
		return nil, err
	}

	containerProvider := worker.containerProviderFactory.ContainerProviderFor(worker)
	return containerProvider.FindOrCreateContainer(logger, cancel, creatingContainer, delegate, id, metadata, spec, resourceTypes, map[string]string{})
}

func (worker *gardenWorker) FindOrCreateContainerForIdentifier(
	logger lager.Logger,
	id Identifier,
	metadata Metadata,
	containerSpec ContainerSpec,
	resourceTypes atc.ResourceTypes,
	imageFetchingDelegate ImageFetchingDelegate,
	resourceSources map[string]ArtifactSource,
) (Container, []string, error) {
	container, err := worker.getContainerForIdentifier(
		logger,
		id,
		metadata,
		containerSpec,
		resourceTypes,
		imageFetchingDelegate,
	)
	if err != nil {
		return nil, nil, err
	}

	return container, nil, nil
}

func (worker *gardenWorker) FindContainerForIdentifier(logger lager.Logger, id Identifier) (Container, bool, error) {
	containerInfo, found, err := worker.provider.FindContainerForIdentifier(id)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, found, nil
	}

	valid, err := worker.ValidateResourceCheckVersion(containerInfo)

	if err != nil {
		return nil, false, err
	}

	if !valid {
		logger.Debug("check-container-version-outdated", lager.Data{
			"container-handle": containerInfo.Handle,
			"worker-name":      containerInfo.WorkerName,
		})

		return nil, false, nil
	}

	container, found, err := worker.LookupContainer(logger, containerInfo.Handle)
	if err != nil {
		return nil, false, err
	}

	if !found {
		logger.Info("reaping-container-not-found-on-worker", lager.Data{
			"container-handle": containerInfo.Handle,
			"worker-name":      containerInfo.WorkerName,
		})

		err := worker.provider.ReapContainer(containerInfo.Handle)
		if err != nil {
			return nil, false, err
		}

		return nil, false, nil
	}

	return container, found, nil
}

func (worker *gardenWorker) LookupContainer(logger lager.Logger, handle string) (Container, bool, error) {
	containerProvider := worker.containerProviderFactory.ContainerProviderFor(worker)
	return containerProvider.FindContainerByHandle(logger, handle)
}

func (worker *gardenWorker) ActiveContainers() int {
	return worker.activeContainers
}

func (worker *gardenWorker) Satisfying(spec WorkerSpec, resourceTypes atc.ResourceTypes) (Worker, error) {
	if spec.TeamID != worker.teamID && worker.teamID != 0 {
		return nil, ErrTeamMismatch
	}

	if spec.ResourceType != "" {
		underlyingType := determineUnderlyingTypeName(spec.ResourceType, resourceTypes)

		matchedType := false
		for _, t := range worker.resourceTypes {
			if t.Type == underlyingType {
				matchedType = true
				break
			}
		}

		if !matchedType {
			return nil, ErrUnsupportedResourceType
		}
	}

	if spec.Platform != "" {
		if spec.Platform != worker.platform {
			return nil, ErrIncompatiblePlatform
		}
	}

	if !worker.tagsMatch(spec.Tags) {
		return nil, ErrMismatchedTags
	}

	return worker, nil
}

func determineUnderlyingTypeName(typeName string, resourceTypes atc.ResourceTypes) string {
	resourceTypesMap := make(map[string]atc.ResourceType)
	for _, resourceType := range resourceTypes {
		resourceTypesMap[resourceType.Name] = resourceType
	}
	underlyingTypeName := typeName
	underlyingType, ok := resourceTypesMap[underlyingTypeName]
	for ok {
		underlyingTypeName = underlyingType.Type
		underlyingType, ok = resourceTypesMap[underlyingTypeName]
		delete(resourceTypesMap, underlyingTypeName)
	}
	return underlyingTypeName
}

func (worker *gardenWorker) AllSatisfying(spec WorkerSpec, resourceTypes atc.ResourceTypes) ([]Worker, error) {
	return nil, ErrNotImplemented
}

func (worker *gardenWorker) Workers() ([]Worker, error) {
	return nil, ErrNotImplemented
}

func (worker *gardenWorker) GetWorker(name string) (Worker, error) {
	return nil, ErrNotImplemented
}

func (worker *gardenWorker) Description() string {
	messages := []string{
		fmt.Sprintf("platform '%s'", worker.platform),
	}

	for _, tag := range worker.tags {
		messages = append(messages, fmt.Sprintf("tag '%s'", tag))
	}

	return strings.Join(messages, ", ")
}

func (worker *gardenWorker) Name() string {
	return worker.name
}

func (worker *gardenWorker) ResourceTypes() []atc.WorkerResourceType {
	return worker.resourceTypes
}

func (worker *gardenWorker) Tags() atc.Tags {
	return worker.tags
}

func (worker *gardenWorker) IsOwnedByTeam() bool {
	return worker.teamID != 0
}

func (worker *gardenWorker) Uptime() time.Duration {
	return worker.clock.Since(time.Unix(worker.startTime, 0))
}

func (worker *gardenWorker) tagsMatch(tags []string) bool {
	if len(worker.tags) > 0 && len(tags) == 0 {
		return false
	}

insert_coin:
	for _, stag := range tags {
		for _, wtag := range worker.tags {
			if stag == wtag {
				continue insert_coin
			}
		}

		return false
	}

	return true
}

func (worker *gardenWorker) getContainerForIdentifier(
	logger lager.Logger,
	id Identifier,
	metadata Metadata,
	resourceSpec ContainerSpec,
	resourceTypes atc.ResourceTypes,
	imageFetchingDelegate ImageFetchingDelegate,
) (Container, error) {
	logger = logger.Session("get-container-for-identifier")

	logger.Debug("start")
	defer logger.Debug("done")

	container, found, err := worker.FindContainerForIdentifier(logger, id)
	if err != nil {
		logger.Error("failed-to-look-for-existing-container", err, lager.Data{"id": id})
		return nil, err
	}

	if found {
		logger.Debug("found-existing-container", lager.Data{"container": container.Handle()})
		return container, nil
	}

	if id.BuildID != 0 {
		container, err = worker.CreateBuildContainer(
			logger,
			nil,
			imageFetchingDelegate,
			id,
			metadata,
			resourceSpec,
			resourceTypes,
			map[string]string{},
		)
	} else if id.ResourceID != 0 {
		container, err = worker.CreateResourceCheckContainer(
			logger,
			nil,
			imageFetchingDelegate,
			id,
			metadata,
			resourceSpec,
			resourceTypes,
			id.CheckType,
			id.CheckSource,
		)
	} else {
		container, err = worker.CreateResourceTypeCheckContainer(
			logger,
			nil,
			imageFetchingDelegate,
			id,
			metadata,
			resourceSpec,
			resourceTypes,
			id.CheckType,
			id.CheckSource,
		)
	}

	if err != nil {
		logger.Error("failed-to-create-container", err)
		return nil, err
	}

	logger.Info("created", lager.Data{"container": container.Handle()})

	return container, nil
}

type artifactDestination struct {
	destination Volume
}

func (wad *artifactDestination) StreamIn(path string, tarStream io.Reader) error {
	return wad.destination.StreamIn(path, tarStream)
}

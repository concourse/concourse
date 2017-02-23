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
var ErrNotImplemented = errors.New("Not implemented")

type MalformedMetadataError struct {
	UnmarshalError error
}

func (err MalformedMetadataError) Error() string {
	return fmt.Sprintf("malformed image metadata: %s", err.UnmarshalError)
}

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
	Address() *string
	ResourceTypes() []atc.WorkerResourceType
	Tags() atc.Tags
	Uptime() time.Duration
	IsOwnedByTeam() bool
}

//go:generate counterfeiter . GardenWorkerDB

type GardenWorkerDB interface {
	CreateContainerToBeRemoved(container db.Container, maxLifetime time.Duration, volumeHandles []string) (db.SavedContainer, error)
	UpdateContainerTTLToBeRemoved(container db.Container, maxLifetime time.Duration) (db.SavedContainer, error)
	GetContainer(handle string) (db.SavedContainer, bool, error)
	ReapContainer(string) error
	GetPipelineByID(pipelineID int) (db.SavedPipeline, error)
	AcquireVolumeCreatingLock(lager.Logger, int) (lock.Lock, bool, error)
	AcquireContainerCreatingLock(lager.Logger, int) (lock.Lock, bool, error)
}

type gardenWorker struct {
	containerProviderFactory ContainerProviderFactory

	volumeClient      VolumeClient
	pipelineDBFactory db.PipelineDBFactory

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
}

func NewGardenWorker(
	containerProviderFactory ContainerProviderFactory,
	volumeClient VolumeClient,
	pipelineDBFactory db.PipelineDBFactory,
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
) Worker {
	return &gardenWorker{
		containerProviderFactory: containerProviderFactory,

		volumeClient: volumeClient,

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

func (worker *gardenWorker) CreateVolumeForResourceCache(logger lager.Logger, volumeSpec VolumeSpec, resourceCache *dbng.UsedResourceCache) (Volume, error) {
	return worker.volumeClient.CreateVolumeForResourceCache(logger, volumeSpec, resourceCache)
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

func (worker *gardenWorker) FindOrCreateBuildContainer(
	logger lager.Logger,
	cancel <-chan os.Signal,
	delegate ImageFetchingDelegate,
	id Identifier,
	metadata Metadata,
	spec ContainerSpec,
	resourceTypes atc.ResourceTypes,
	outputPaths map[string]string,
) (Container, error) {
	containerProvider := worker.containerProviderFactory.ContainerProviderFor(worker)

	return containerProvider.FindOrCreateBuildContainer(
		logger,
		cancel,
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
	containerProvider := worker.containerProviderFactory.ContainerProviderFor(worker)
	return containerProvider.FindOrCreateResourceGetContainer(
		logger,
		cancel,
		delegate,
		id,
		metadata,
		spec,
		resourceTypes,
		outputPaths,
		resourceTypeName,
		version,
		source,
		params,
	)
}

func (worker *gardenWorker) FindOrCreateResourceCheckContainer(
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
	containerProvider := worker.containerProviderFactory.ContainerProviderFor(worker)

	return containerProvider.FindOrCreateResourceCheckContainer(
		logger,
		cancel,
		delegate,
		id,
		metadata,
		spec,
		resourceTypes,
		resourceType,
		source,
	)
}

func (worker *gardenWorker) FindOrCreateResourceTypeCheckContainer(
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
	containerProvider := worker.containerProviderFactory.ContainerProviderFor(worker)

	return containerProvider.FindOrCreateResourceTypeCheckContainer(
		logger,
		cancel,
		delegate,
		id,
		metadata,
		spec,
		resourceTypes,
		resourceTypeName,
		source,
	)
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
	container, err := worker.findOrCreateContainerForIdentifier(
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

	container, found, err := worker.FindContainerByHandle(logger, containerInfo.Handle, containerInfo.TeamID)
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

func (worker *gardenWorker) FindContainerByHandle(logger lager.Logger, handle string, teamID int) (Container, bool, error) {
	containerProvider := worker.containerProviderFactory.ContainerProviderFor(worker)
	return containerProvider.FindContainerByHandle(logger, handle, teamID)
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

func (worker *gardenWorker) RunningWorkers() ([]Worker, error) {
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

func (worker *gardenWorker) Address() *string {
	return &worker.addr
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

func (worker *gardenWorker) findOrCreateContainerForIdentifier(
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

	logger.Debug("creating-container-for-identifier", lager.Data{"id": id})
	if id.BuildID != 0 {
		container, err = worker.FindOrCreateBuildContainer(
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
		container, err = worker.FindOrCreateResourceCheckContainer(
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
		container, err = worker.FindOrCreateResourceTypeCheckContainer(
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

package worker

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

var ErrUnsupportedResourceType = errors.New("unsupported resource type")
var ErrIncompatiblePlatform = errors.New("incompatible platform")
var ErrMismatchedTags = errors.New("mismatched tags")

const containerKeepalive = 30 * time.Second
const containerTTL = 5 * time.Minute
const VolumeTTL = containerTTL

//go:generate counterfeiter . Worker

type Worker interface {
	Client

	ActiveContainers() int

	Description() string
	Name() string

	VolumeManager() (baggageclaim.Client, bool)
}

//go:generate counterfeiter . GardenWorkerDB

type GardenWorkerDB interface {
	CreateContainer(db.Container, time.Duration) (db.Container, error)
	UpdateExpiresAtOnContainer(handle string, ttl time.Duration) error
}

type gardenWorker struct {
	gardenClient       garden.Client
	baggageclaimClient baggageclaim.Client
	volumeFactory      VolumeFactory

	imageFetcher ImageFetcher

	db       GardenWorkerDB
	provider WorkerProvider

	clock clock.Clock

	activeContainers int
	resourceTypes    []atc.WorkerResourceType
	platform         string
	tags             []string
	name             string
}

func NewGardenWorker(
	gardenClient garden.Client,
	baggageclaimClient baggageclaim.Client,
	volumeFactory VolumeFactory,
	imageFetcher ImageFetcher,
	db GardenWorkerDB,
	provider WorkerProvider,
	clock clock.Clock,
	activeContainers int,
	resourceTypes []atc.WorkerResourceType,
	platform string,
	tags []string,
	name string,
) Worker {
	return &gardenWorker{
		gardenClient:       gardenClient,
		baggageclaimClient: baggageclaimClient,
		volumeFactory:      volumeFactory,
		imageFetcher:       imageFetcher,
		db:                 db,
		provider:           provider,
		clock:              clock,

		activeContainers: activeContainers,
		resourceTypes:    resourceTypes,
		platform:         platform,
		tags:             tags,
		name:             name,
	}
}

func (worker *gardenWorker) VolumeManager() (baggageclaim.Client, bool) {
	if worker.baggageclaimClient != nil {
		return worker.baggageclaimClient, true
	}

	return nil, false
}

func (worker *gardenWorker) CreateContainer(
	logger lager.Logger,
	cancel <-chan os.Signal,
	delegate ImageFetchingDelegate,
	id Identifier,
	metadata Metadata,
	spec ContainerSpec,
	customTypes atc.ResourceTypes,
) (Container, error) {
	gardenContainerSpecFactory := NewGardenContainerSpecFactory(logger, worker.baggageclaimClient, worker.imageFetcher)

	gardenSpec, err := gardenContainerSpecFactory.BuildContainerSpec(spec, worker.resourceTypes, cancel, delegate, id, metadata, worker, customTypes)
	defer gardenContainerSpecFactory.ReleaseVolumes()
	if err != nil {
		return nil, err
	}

	gardenContainer, err := worker.gardenClient.Create(gardenSpec)
	if err != nil {
		return nil, err
	}

	metadata.WorkerName = worker.name
	metadata.Handle = gardenContainer.Handle()
	metadata.User = gardenSpec.Properties["user"]
	_, err = worker.db.CreateContainer(
		db.Container{
			ContainerIdentifier: db.ContainerIdentifier(id),
			ContainerMetadata:   db.ContainerMetadata(metadata),
		},
		containerTTL,
	)
	if err != nil {
		return nil, err
	}

	return newGardenWorkerContainer(
		logger,
		gardenContainer,
		worker.gardenClient,
		worker.baggageclaimClient,
		worker.db,
		worker.clock,
		worker.volumeFactory,
	)
}

func (worker *gardenWorker) FindContainerForIdentifier(logger lager.Logger, id Identifier) (Container, bool, error) {
	containerInfo, found, err := worker.provider.FindContainerForIdentifier(id)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, found, nil
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

		return nil, false, err
	}

	return container, found, nil
}

func (worker *gardenWorker) LookupContainer(logger lager.Logger, handle string) (Container, bool, error) {
	gardenContainer, err := worker.gardenClient.Lookup(handle)
	if err != nil {
		if _, ok := err.(garden.ContainerNotFoundError); ok {
			logger.Info("container-not-found")
			return nil, false, nil
		}

		logger.Error("failed-to-lookup-on-garden", err)
		return nil, false, err
	}

	container, err := newGardenWorkerContainer(
		logger,
		gardenContainer,
		worker.gardenClient,
		worker.baggageclaimClient,
		worker.db,
		worker.clock,
		worker.volumeFactory,
	)
	if err != nil {
		logger.Error("failed-to-construct-container", err)
		return nil, false, err
	}

	return container, true, nil
}

func (worker *gardenWorker) ActiveContainers() int {
	return worker.activeContainers
}

func (worker *gardenWorker) Satisfying(spec WorkerSpec, resourceTypes atc.ResourceTypes) (Worker, error) {
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
	return nil, errors.New("Not implemented")
}

func (worker *gardenWorker) GetWorker(name string) (Worker, error) {
	return nil, errors.New("Not implemented")
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

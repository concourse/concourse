package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
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
var ErrNoVolumeManager = errors.New("worker does not support volume management")

const containerKeepalive = 30 * time.Second
const ContainerTTL = 5 * time.Minute
const VolumeTTL = 5 * time.Minute

const ephemeralPropertyName = "concourse:ephemeral"
const volumePropertyName = "concourse:volumes"
const volumeMountsPropertyName = "concourse:volume-mounts"
const userPropertyName = "user"
const RawRootFSScheme = "raw"

//go:generate counterfeiter . Worker

type Worker interface {
	Client

	ActiveContainers() int

	Description() string
	Name() string
	Uptime() time.Duration
}

//go:generate counterfeiter . GardenWorkerDB

type GardenWorkerDB interface {
	CreateContainer(db.Container, time.Duration, time.Duration) (db.SavedContainer, error)
	UpdateExpiresAtOnContainer(handle string, ttl time.Duration) error

	InsertVolume(db.Volume) error
	SetVolumeTTL(string, time.Duration) error
	GetVolumeTTL(string) (time.Duration, bool, error)
	GetVolumesByIdentifier(db.VolumeIdentifier) ([]db.SavedVolume, error)
	ReapVolume(string) error
}

type gardenWorker struct {
	gardenClient       garden.Client
	baggageclaimClient baggageclaim.Client
	volumeClient       VolumeClient
	volumeFactory      VolumeFactory

	imageFetcher ImageFetcher

	db       GardenWorkerDB
	provider WorkerProvider

	clock clock.Clock

	activeContainers int
	resourceTypes    []atc.WorkerResourceType
	platform         string
	tags             atc.Tags
	name             string
	startTime        int64
	httpProxyURL     string
	httpsProxyURL    string
	noProxy          string
}

func NewGardenWorker(
	gardenClient garden.Client,
	baggageclaimClient baggageclaim.Client,
	volumeClient VolumeClient,
	volumeFactory VolumeFactory,
	imageFetcher ImageFetcher,
	db GardenWorkerDB,
	provider WorkerProvider,
	clock clock.Clock,
	activeContainers int,
	resourceTypes []atc.WorkerResourceType,
	platform string,
	tags atc.Tags,
	name string,
	startTime int64,
	httpProxyURL string,
	httpsProxyURL string,
	noProxy string,
) Worker {
	return &gardenWorker{
		gardenClient:       gardenClient,
		baggageclaimClient: baggageclaimClient,
		volumeClient:       volumeClient,
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
		startTime:        startTime,
		httpProxyURL:     httpProxyURL,
		httpsProxyURL:    httpsProxyURL,
		noProxy:          noProxy,
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

func (worker *gardenWorker) FindVolume(logger lager.Logger, volumeSpec VolumeSpec) (Volume, bool, error) {
	return worker.volumeClient.FindVolume(logger, volumeSpec)
}

func (worker *gardenWorker) CreateVolume(logger lager.Logger, volumeSpec VolumeSpec) (Volume, error) {
	return worker.volumeClient.CreateVolume(logger, volumeSpec)
}

func (worker *gardenWorker) ListVolumes(logger lager.Logger, properties VolumeProperties) ([]Volume, error) {
	return worker.volumeClient.ListVolumes(logger, properties)
}

func (worker *gardenWorker) LookupVolume(logger lager.Logger, handle string) (Volume, bool, error) {
	return worker.volumeClient.LookupVolume(logger, handle)
}

func (worker *gardenWorker) getImage(
	logger lager.Logger,
	imageSpec ImageSpec,
	cancel <-chan os.Signal,
	delegate ImageFetchingDelegate,
	id Identifier,
	metadata Metadata,
	resourceTypes atc.ResourceTypes,
) (Image, error) {
	updatedResourceTypes := resourceTypes
	imageResource := imageSpec.ImageResource
	for _, resourceType := range resourceTypes {
		if resourceType.Name == imageSpec.ResourceType {
			updatedResourceTypes = resourceTypes.Without(imageSpec.ResourceType)
			imageResource = &atc.ImageResource{
				Source: resourceType.Source,
				Type:   resourceType.Type,
			}
		}
	}

	if imageResource != nil {
		return worker.imageFetcher.FetchImage(
			logger,
			*imageResource,
			cancel,
			id,
			metadata,
			delegate,
			worker,
			worker.tags,
			updatedResourceTypes,
			imageSpec.Privileged,
		)
	}

	if imageSpec.ResourceType != "" {
		rootFSURL, volume, err := worker.getBuiltInResourceTypeImage(logger, imageSpec.ResourceType)
		if err != nil {
			return nil, err
		}

		return &dummyImage{url: rootFSURL, volume: volume}, nil
	}

	return &dummyImage{url: imageSpec.ImageURL, volume: nil}, nil
}

type dummyImage struct {
	url    string
	volume Volume
}

func (im *dummyImage) URL() string {
	return im.url
}

func (im *dummyImage) Volume() Volume {
	return im.volume
}

func (im *dummyImage) Release(ttl *time.Duration) {
	if im.volume != nil {
		im.volume.Release(ttl)
	}
}

func (im *dummyImage) Metadata() ImageMetadata {
	return ImageMetadata{}
}

func (im *dummyImage) Version() atc.Version {
	return nil
}

func (worker *gardenWorker) getBuiltInResourceTypeImage(
	logger lager.Logger,
	resourceTypeName string,
) (string, Volume, error) {
	for _, t := range worker.resourceTypes {
		if t.Type == resourceTypeName {
			importVolumeSpec := VolumeSpec{
				Strategy: HostRootFSStrategy{
					Path:       t.Image,
					Version:    &t.Version,
					WorkerName: worker.Name(),
				},
				Privileged: true,
				Properties: VolumeProperties{},
				TTL:        0,
			}

			importVolume, found, err := worker.FindVolume(logger, importVolumeSpec)
			if !found || err != nil {
				importVolume, err = worker.CreateVolume(logger, importVolumeSpec)
				if err != nil {
					return "", nil, err
				}
			}
			defer importVolume.Release(nil)

			cowVolume, err := worker.CreateVolume(logger, VolumeSpec{
				Strategy: ContainerRootFSStrategy{
					Parent: importVolume,
				},
				Privileged: true,
				Properties: VolumeProperties{},
				TTL:        VolumeTTL,
			})
			if err != nil {
				return "", nil, err
			}

			rootFSURL := url.URL{
				Scheme: RawRootFSScheme,
				Path:   cowVolume.Path(),
			}

			return rootFSURL.String(), cowVolume, nil
		}
	}

	return "", nil, ErrUnsupportedResourceType
}

func (worker *gardenWorker) CreateContainer(
	logger lager.Logger,
	cancel <-chan os.Signal,
	delegate ImageFetchingDelegate,
	id Identifier,
	metadata Metadata,
	spec ContainerSpec,
	resourceTypes atc.ResourceTypes,
) (Container, error) {
	image, err := worker.getImage(
		logger,
		spec.ImageSpec,
		cancel,
		delegate,
		id,
		metadata,
		resourceTypes,
	)
	if err != nil {
		return nil, err
	}

	defer image.Release(nil)

	id.ResourceTypeVersion = image.Version()

	volumeMounts := spec.Outputs
	for _, mount := range spec.Inputs {
		cowVolume, err := worker.CreateVolume(logger, VolumeSpec{
			Strategy: ContainerRootFSStrategy{
				Parent: mount.Volume,
			},
			Privileged: spec.ImageSpec.Privileged,
			TTL:        VolumeTTL,
		})
		if err != nil {
			return nil, err
		}
		// release *after* container creation
		defer cowVolume.Release(nil)

		volumeMounts = append(volumeMounts, VolumeMount{
			Volume:    cowVolume,
			MountPath: mount.MountPath,
		})

		logger.Debug("created-cow-volume", lager.Data{
			"original-volume-handle": mount.Volume.Handle(),
			"cow-volume-handle":      cowVolume.Handle(),
		})
	}

	bindMounts := []garden.BindMount{}
	volumeHandles := []string{}
	volumeHandleMounts := map[string]string{}
	for _, mount := range volumeMounts {
		volumeHandles = append(volumeHandles, mount.Volume.Handle())
		bindMounts = append(bindMounts, garden.BindMount{
			SrcPath: mount.Volume.Path(),
			DstPath: mount.MountPath,
			Mode:    garden.BindMountModeRW,
		})
		volumeHandleMounts[mount.Volume.Handle()] = mount.MountPath
	}

	if image.Volume() != nil {
		volumeHandles = append(volumeHandles, image.Volume().Handle())
	}

	gardenProperties := garden.Properties{userPropertyName: image.Metadata().User}

	if len(volumeHandles) > 0 {
		volumesJSON, err := json.Marshal(volumeHandles)
		if err != nil {
			return nil, err
		}

		gardenProperties[volumePropertyName] = string(volumesJSON)

		mountsJSON, err := json.Marshal(volumeHandleMounts)
		if err != nil {
			return nil, err
		}

		gardenProperties[volumeMountsPropertyName] = string(mountsJSON)
	}

	if spec.Ephemeral {
		gardenProperties[ephemeralPropertyName] = "true"
	}

	env := append(image.Metadata().Env, spec.Env...)

	if worker.httpProxyURL != "" {
		env = append(env, fmt.Sprintf("http_proxy=%s", worker.httpProxyURL))
	}

	if worker.httpsProxyURL != "" {
		env = append(env, fmt.Sprintf("https_proxy=%s", worker.httpsProxyURL))
	}

	if worker.noProxy != "" {
		env = append(env, fmt.Sprintf("no_proxy=%s", worker.noProxy))
	}

	gardenSpec := garden.ContainerSpec{
		BindMounts: bindMounts,
		Privileged: spec.ImageSpec.Privileged,
		Properties: gardenProperties,
		RootFSPath: image.URL(),
		Env:        env,
	}

	gardenContainer, err := worker.gardenClient.Create(gardenSpec)
	if err != nil {
		return nil, err
	}

	metadata.WorkerName = worker.name
	metadata.Handle = gardenContainer.Handle()
	metadata.User = gardenSpec.Properties["user"]

	id.ResourceTypeVersion = image.Version()

	_, err = worker.db.CreateContainer(
		db.Container{
			ContainerIdentifier: db.ContainerIdentifier(id),
			ContainerMetadata:   db.ContainerMetadata(metadata),
		},
		ContainerTTL,
		worker.maxContainerLifetime(metadata),
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

func (worker *gardenWorker) maxContainerLifetime(metadata Metadata) time.Duration {
	if metadata.Type == db.ContainerTypeCheck {
		uptime := worker.Uptime()
		switch {
		case uptime < 5*time.Minute:
			return 5 * time.Minute
		case uptime > 1*time.Hour:
			return 1 * time.Hour
		default:
			return uptime
		}
	}

	return time.Duration(0)
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

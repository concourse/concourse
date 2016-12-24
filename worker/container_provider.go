package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/baggageclaim"
)

var ErrCreatedContainerNotFound = errors.New("container-in-created-state-not-found-in-garden")

const creatingContainerRetryDelay = 1 * time.Second

//go:generate counterfeiter . ContainerProviderFactory

type ContainerProviderFactory interface {
	ContainerProviderFor(Worker) ContainerProvider
}

type containerProviderFactory struct {
	gardenClient            garden.Client
	baggageclaimClient      baggageclaim.Client
	volumeClient            VolumeClient
	imageFactory            ImageFactory
	dbContainerFactory      dbng.ContainerFactory
	dbVolumeFactory         dbng.VolumeFactory
	dbResourceCacheFactory  dbng.ResourceCacheFactory
	dbResourceConfigFactory dbng.ResourceConfigFactory

	db GardenWorkerDB

	httpProxyURL  string
	httpsProxyURL string
	noProxy       string

	clock clock.Clock
}

func NewContainerProviderFactory(
	gardenClient garden.Client,
	baggageclaimClient baggageclaim.Client,
	volumeClient VolumeClient,
	imageFactory ImageFactory,
	dbContainerFactory dbng.ContainerFactory,
	dbVolumeFactory dbng.VolumeFactory,
	dbResourceCacheFactory dbng.ResourceCacheFactory,
	dbResourceConfigFactory dbng.ResourceConfigFactory,
	db GardenWorkerDB,
	httpProxyURL string,
	httpsProxyURL string,
	noProxy string,
	clock clock.Clock,
) ContainerProviderFactory {
	return &containerProviderFactory{
		gardenClient:            gardenClient,
		baggageclaimClient:      baggageclaimClient,
		volumeClient:            volumeClient,
		imageFactory:            imageFactory,
		dbContainerFactory:      dbContainerFactory,
		dbVolumeFactory:         dbVolumeFactory,
		dbResourceCacheFactory:  dbResourceCacheFactory,
		dbResourceConfigFactory: dbResourceConfigFactory,
		db:            db,
		httpProxyURL:  httpProxyURL,
		httpsProxyURL: httpsProxyURL,
		noProxy:       noProxy,
		clock:         clock,
	}
}

func (f *containerProviderFactory) ContainerProviderFor(
	worker Worker,
) ContainerProvider {
	return &containerProvider{
		gardenClient:            f.gardenClient,
		baggageclaimClient:      f.baggageclaimClient,
		volumeClient:            f.volumeClient,
		imageFactory:            f.imageFactory,
		dbContainerFactory:      f.dbContainerFactory,
		dbVolumeFactory:         f.dbVolumeFactory,
		dbResourceCacheFactory:  f.dbResourceCacheFactory,
		dbResourceConfigFactory: f.dbResourceConfigFactory,
		db:            f.db,
		httpProxyURL:  f.httpProxyURL,
		httpsProxyURL: f.httpsProxyURL,
		noProxy:       f.noProxy,
		clock:         f.clock,
		worker:        worker,
	}
}

//go:generate counterfeiter . ContainerProvider

type ContainerProvider interface {
	FindContainerByHandle(
		logger lager.Logger,
		handle string,
	) (Container, bool, error)

	FindOrCreateBuildContainer(
		logger lager.Logger,
		cancel <-chan os.Signal,
		delegate ImageFetchingDelegate,
		id Identifier,
		metadata Metadata,
		spec ContainerSpec,
		resourceTypes atc.ResourceTypes,
		outputPaths map[string]string,
	) (Container, error)

	FindOrCreateResourceCheckContainer(
		logger lager.Logger,
		cancel <-chan os.Signal,
		delegate ImageFetchingDelegate,
		id Identifier,
		metadata Metadata,
		spec ContainerSpec,
		resourceTypes atc.ResourceTypes,
		resourceType string,
		source atc.Source,
	) (Container, error)

	FindOrCreateResourceTypeCheckContainer(
		logger lager.Logger,
		cancel <-chan os.Signal,
		delegate ImageFetchingDelegate,
		id Identifier,
		metadata Metadata,
		spec ContainerSpec,
		resourceTypes atc.ResourceTypes,
		resourceTypeName string,
		source atc.Source,
	) (Container, error)

	FindOrCreateResourceGetContainer(
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
	) (Container, error)
}

type containerProvider struct {
	gardenClient            garden.Client
	baggageclaimClient      baggageclaim.Client
	volumeClient            VolumeClient
	imageFactory            ImageFactory
	dbContainerFactory      dbng.ContainerFactory
	dbVolumeFactory         dbng.VolumeFactory
	dbResourceCacheFactory  dbng.ResourceCacheFactory
	dbResourceConfigFactory dbng.ResourceConfigFactory

	db       GardenWorkerDB
	provider WorkerProvider

	worker        Worker
	httpProxyURL  string
	httpsProxyURL string
	noProxy       string

	clock clock.Clock
}

// TODO split this method into different methods:
// FindOrCreateBuildContainer, FindOrCreateResourceCheckContainer, FindOrCreateResourceGetContainer, FindOrCreateResourceTypeCheckContainer
// (called <METHODS> bellow)
//
// * private findOrCreateContainer takes in also funcs to find and create in dbng separately.
// So these methods will be different depending on what this container is created for.
// See volume_client how findOrCreateVolume is used and different dbng methods for find or create are provided
// depending what the volume is being created for.
//
// * use <METHODS> in worker/worker instead of FindOrCreateContainer appropriately
// worker/worker already has separate methods for containers so move logic for these methods from worker/worker down here
// see how volume_client is used in worker/worker
//
// * dbng ContainerFactory has methods to create containers for different purposed but not find.
// See dbng.VolumeFactory how each create has a corresponding find
//
// * make sure testflight pass before fixing unit tests
//
// p.findOrCreateContainer has no unit test coverage. Please add. See volume_client_test how the behaviour is tested for volumes
// different cases, e.g. container found in DB but not found in garden. If it is created container we raise error.
// If it is creating container, that means that ATC might have failed after container was created in DB but not in garden.
// So create in garden. Mark as created. We use lock so that if two threads find it in this state only one will create in garden.
//
// * have fun!

func (p *containerProvider) FindOrCreateBuildContainer(
	logger lager.Logger,
	cancel <-chan os.Signal,
	delegate ImageFetchingDelegate,
	id Identifier,
	metadata Metadata,
	spec ContainerSpec,
	resourceTypes atc.ResourceTypes,
	outputPaths map[string]string,
) (Container, error) {
	return p.findOrCreateContainer(
		logger,
		cancel,
		delegate,
		id,
		metadata,
		spec,
		resourceTypes,
		outputPaths,
		func() (dbng.CreatingContainer, dbng.CreatedContainer, error) {
			return nil, nil, nil
		},
		func() (dbng.CreatingContainer, error) {
			return p.dbContainerFactory.CreateBuildContainer(
				&dbng.Worker{
					Name:       p.worker.Name(),
					GardenAddr: p.worker.Address(),
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
		},
	)
}

func (p *containerProvider) FindOrCreateResourceCheckContainer(
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
	return p.findOrCreateContainer(
		logger,
		cancel,
		delegate,
		id,
		metadata,
		spec,
		resourceTypes,
		map[string]string{},
		func() (dbng.CreatingContainer, dbng.CreatedContainer, error) {
			return nil, nil, nil
		},
		func() (dbng.CreatingContainer, error) {
			resourceConfig, err := p.dbResourceConfigFactory.FindOrCreateResourceConfigForResource(
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

			return p.dbContainerFactory.CreateResourceCheckContainer(
				&dbng.Worker{
					Name:       p.worker.Name(),
					GardenAddr: p.worker.Address(),
				},
				resourceConfig,
			)
		},
	)
}

func (p *containerProvider) FindOrCreateResourceTypeCheckContainer(
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
	return p.findOrCreateContainer(
		logger,
		cancel,
		delegate,
		id,
		metadata,
		spec,
		resourceTypes,
		map[string]string{},
		func() (dbng.CreatingContainer, dbng.CreatedContainer, error) {
			return nil, nil, nil
		},
		func() (dbng.CreatingContainer, error) {
			resourceConfig, err := p.dbResourceConfigFactory.FindOrCreateResourceConfigForResourceType(
				logger,
				resourceTypeName,
				source,
				&dbng.Pipeline{ID: metadata.PipelineID},
				resourceTypes,
			)
			if err != nil {
				return nil, err
			}

			return p.dbContainerFactory.CreateResourceCheckContainer(
				&dbng.Worker{
					Name:       p.worker.Name(),
					GardenAddr: p.worker.Address(),
				},
				resourceConfig,
			)
		},
	)
}

func (p *containerProvider) FindOrCreateResourceGetContainer(
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
	return p.findOrCreateContainer(
		logger,
		cancel,
		delegate,
		id,
		metadata,
		spec,
		resourceTypes,
		map[string]string{},
		func() (dbng.CreatingContainer, dbng.CreatedContainer, error) {
			return nil, nil, nil
		},
		func() (dbng.CreatingContainer, error) {
			var resourceCache *dbng.UsedResourceCache

			if id.BuildID != 0 {
				var err error
				resourceCache, err = p.dbResourceCacheFactory.FindOrCreateResourceCacheForBuild(
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
				resourceCache, err = p.dbResourceCacheFactory.FindOrCreateResourceCacheForResource(
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
				resourceCache, err = p.dbResourceCacheFactory.FindOrCreateResourceCacheForResourceType(
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

			return p.dbContainerFactory.CreateResourceGetContainer(
				&dbng.Worker{
					Name:       p.worker.Name(),
					GardenAddr: p.worker.Address(),
				},
				resourceCache,
				metadata.StepName,
			)
		},
	)
}

func (p *containerProvider) FindContainerByHandle(
	logger lager.Logger,
	handle string,
) (Container, bool, error) {
	gardenContainer, err := p.gardenClient.Lookup(handle)
	if err != nil {
		if _, ok := err.(garden.ContainerNotFoundError); ok {
			logger.Info("container-not-found")
			return nil, false, nil
		}

		logger.Error("failed-to-lookup-on-garden", err)
		return nil, false, err
	}

	createdContainer, found, err := p.dbContainerFactory.FindContainerByHandle(handle)
	if err != nil {
		logger.Error("failed-to-lookup-in-db", err)
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	createdVolumes, err := p.dbVolumeFactory.FindVolumesForContainer(createdContainer)
	if err != nil {
		return nil, false, err
	}

	container, err := newGardenWorkerContainer(
		logger,
		gardenContainer,
		createdContainer,
		createdVolumes,
		p.gardenClient,
		p.baggageclaimClient,
		p.db,
		p.worker.Name(),
	)

	if err != nil {
		logger.Error("failed-to-construct-container", err)
		return nil, false, err
	}

	return container, true, nil
}

func (p *containerProvider) findOrCreateContainer(
	logger lager.Logger,
	cancel <-chan os.Signal,
	delegate ImageFetchingDelegate,
	id Identifier,
	metadata Metadata,
	spec ContainerSpec,
	resourceTypes atc.ResourceTypes,
	outputPaths map[string]string,
	findContainerFunc func() (dbng.CreatingContainer, dbng.CreatedContainer, error),
	createContainerFunc func() (dbng.CreatingContainer, error),
) (Container, error) {
	var gardenContainer garden.Container

	creatingContainer, createdContainer, err := findContainerFunc()
	if err != nil {
		logger.Error("failed-to-find-container-in-db", err)
		return nil, err
	}

	if createdContainer != nil {
		gardenContainer, err = p.gardenClient.Lookup(createdContainer.Handle())
		if err != nil {
			logger.Error("failed-to-lookup-created-container-in-garden", err)
			return nil, err
		}

		createdVolumes, err := p.dbVolumeFactory.FindVolumesForContainer(createdContainer)
		if err != nil {
			logger.Error("failed-to-find-container-volumes", err)
			return nil, err
		}

		return newGardenWorkerContainer(
			logger,
			gardenContainer,
			createdContainer,
			createdVolumes,
			p.gardenClient,
			p.baggageclaimClient,
			p.db,
			p.worker.Name(),
		)
	} else {
		if creatingContainer != nil {
			gardenContainer, err = p.gardenClient.Lookup(createdContainer.Handle())
			if err != nil {
				if _, ok := err.(garden.ContainerNotFoundError); !ok {
					logger.Error("failed-to-lookup-creating-container-in-garden", err)
					return nil, err
				}
			}
		} else {
			creatingContainer, err = createContainerFunc()
			if err != nil {
				logger.Error("failed-to-create-container-in-db", err)
				return nil, err
			}
		}

		lock, acquired, err := p.db.AcquireContainerCreatingLock(logger, creatingContainer.ID())
		if err != nil {
			logger.Error("failed-to-acquire-volume-creating-lock", err)
			return nil, err
		}

		if !acquired {
			p.clock.Sleep(creatingContainerRetryDelay)
			return p.findOrCreateContainer(
				logger,
				cancel,
				delegate,
				id,
				metadata,
				spec,
				resourceTypes,
				outputPaths,
				findContainerFunc,
				createContainerFunc,
			)
		}

		defer lock.Release()

		if gardenContainer == nil {
			logger.Debug("creating-container-in-garden", lager.Data{"handle": creatingContainer.Handle()})
			imageMetadata, imageResourceTypeVersion, imageURL, err := p.getImageForContainer(
				logger,
				creatingContainer,
				spec.ImageSpec,
				spec.TeamID,
				cancel,
				delegate,
				id,
				metadata,
				resourceTypes,
			)
			if err != nil {
				return nil, err
			}

			gardenContainer, err = p.createGardenContainer(
				logger,
				creatingContainer,
				id,
				metadata,
				spec,
				outputPaths,
				imageMetadata,
				imageURL,
			)
			if err != nil {
				logger.Error("failed-to-create-container-in-garden", err)
				return nil, err
			}

			metadata.WorkerName = p.worker.Name()
			metadata.Handle = gardenContainer.Handle()

			metadata.User = imageMetadata.User
			if spec.User != "" {
				metadata.User = spec.User
			}

			id.ResourceTypeVersion = imageResourceTypeVersion

			_, err = p.db.UpdateContainerTTLToBeRemoved(
				db.Container{
					ContainerIdentifier: db.ContainerIdentifier(id),
					ContainerMetadata:   db.ContainerMetadata(metadata),
				},
				p.maxContainerLifetime(metadata),
			)
			if err != nil {
				logger.Error("failed-to-update-container-ttl", err)
				return nil, err
			}
		}

		createdContainer, err = creatingContainer.Created()
		if err != nil {
			logger.Error("failed-to-mark-container-as-created", err)
			return nil, err
		}
	}

	createdVolumes, err := p.dbVolumeFactory.FindVolumesForContainer(createdContainer)
	if err != nil {
		logger.Error("failed-to-find-container-volumes", err)
		return nil, err
	}

	return newGardenWorkerContainer(
		logger,
		gardenContainer,
		createdContainer,
		createdVolumes,
		p.gardenClient,
		p.baggageclaimClient,
		p.db,
		p.worker.Name(),
	)
}

func (p *containerProvider) createGardenContainer(
	logger lager.Logger,
	creatingContainer dbng.CreatingContainer,
	id Identifier,
	metadata Metadata,
	spec ContainerSpec,
	outputPaths map[string]string,
	imageMetadata ImageMetadata,
	imageURL string,
) (garden.Container, error) {
	volumeMounts := []VolumeMount{}
	for name, outputPath := range outputPaths {
		outVolume, volumeErr := p.volumeClient.FindOrCreateVolumeForContainer(
			logger,
			VolumeSpec{
				Strategy:   OutputStrategy{Name: name},
				Privileged: bool(spec.ImageSpec.Privileged),
			},
			creatingContainer,
			&dbng.Team{ID: spec.TeamID},
			outputPath,
		)
		if volumeErr != nil {
			return nil, volumeErr
		}

		volumeMounts = append(volumeMounts, VolumeMount{
			Volume:    outVolume,
			MountPath: outputPath,
		})
	}

	for _, mount := range spec.Mounts {
		volumeMounts = append(volumeMounts, mount)
	}

	for _, mount := range spec.Inputs {
		cowVolume, volumeErr := p.volumeClient.FindOrCreateVolumeForContainer(
			logger,
			VolumeSpec{
				Strategy: ContainerRootFSStrategy{
					Parent: mount.Volume,
				},
				Privileged: spec.ImageSpec.Privileged,
			},
			creatingContainer,
			&dbng.Team{ID: spec.TeamID},
			mount.MountPath,
		)
		if volumeErr != nil {
			return nil, volumeErr
		}

		volumeMounts = append(volumeMounts, VolumeMount{
			Volume:    cowVolume,
			MountPath: mount.MountPath,
		})
	}

	bindMounts := []garden.BindMount{}

	volumeHandleMounts := map[string]string{}
	for _, mount := range volumeMounts {
		bindMounts = append(bindMounts, garden.BindMount{
			SrcPath: mount.Volume.Path(),
			DstPath: mount.MountPath,
			Mode:    garden.BindMountModeRW,
		})
		volumeHandleMounts[mount.Volume.Handle()] = mount.MountPath
	}

	gardenProperties := garden.Properties{userPropertyName: imageMetadata.User}
	if spec.User != "" {
		gardenProperties = garden.Properties{userPropertyName: spec.User}
	}

	if spec.Ephemeral {
		gardenProperties[ephemeralPropertyName] = "true"
	}

	env := append(imageMetadata.Env, spec.Env...)

	if p.httpProxyURL != "" {
		env = append(env, fmt.Sprintf("http_proxy=%s", p.httpProxyURL))
	}

	if p.httpsProxyURL != "" {
		env = append(env, fmt.Sprintf("https_proxy=%s", p.httpsProxyURL))
	}

	if p.noProxy != "" {
		env = append(env, fmt.Sprintf("no_proxy=%s", p.noProxy))
	}

	gardenSpec := garden.ContainerSpec{
		BindMounts: bindMounts,
		Privileged: spec.ImageSpec.Privileged,
		Properties: gardenProperties,
		RootFSPath: imageURL,
		Env:        env,
		Handle:     creatingContainer.Handle(),
	}

	return p.gardenClient.Create(gardenSpec)
}

func (p *containerProvider) getImageForContainer(
	logger lager.Logger,
	container dbng.CreatingContainer,
	imageSpec ImageSpec,
	teamID int,
	cancel <-chan os.Signal,
	delegate ImageFetchingDelegate,
	id Identifier,
	metadata Metadata,
	resourceTypes atc.ResourceTypes,
) (ImageMetadata, atc.Version, string, error) {
	// convert custom resource type from pipeline config into image_resource
	imageResource := imageSpec.ImageResource
	for _, resourceType := range resourceTypes {
		if resourceType.Name == imageSpec.ResourceType {
			imageResource = &atc.ImageResource{
				Source: resourceType.Source,
				Type:   resourceType.Type,
			}
		}
	}

	var imageVolume Volume
	var imageMetadataReader io.ReadCloser
	var version atc.Version

	// image artifact produced by previous step in pipeline
	if imageSpec.ImageArtifactSource != nil {
		artifactVolume, existsOnWorker, err := imageSpec.ImageArtifactSource.VolumeOn(p.worker)
		if err != nil {
			logger.Error("failed-to-check-if-volume-exists-on-worker", err)
			return ImageMetadata{}, nil, "", err
		}

		if existsOnWorker {
			imageVolume, err = p.volumeClient.FindOrCreateVolumeForContainer(
				logger,
				VolumeSpec{
					Strategy: ContainerRootFSStrategy{
						Parent: artifactVolume,
					},
					Privileged: imageSpec.Privileged,
				},
				container,
				&dbng.Team{ID: teamID},
				"/",
			)
			if err != nil {
				logger.Error("failed-to-create-image-artifact-cow-volume", err)
				return ImageMetadata{}, nil, "", err
			}
		} else {
			imageVolume, err = p.volumeClient.FindOrCreateVolumeForContainer(
				logger,
				VolumeSpec{
					Strategy: ImageArtifactReplicationStrategy{
						Name: string(imageSpec.ImageArtifactName),
					},
					Privileged: imageSpec.Privileged,
				},
				container,
				&dbng.Team{ID: teamID},
				"/",
			)
			if err != nil {
				logger.Error("failed-to-create-image-artifact-replicated-volume", err)
				return ImageMetadata{}, nil, "", err
			}

			dest := artifactDestination{
				destination: imageVolume,
			}

			err = imageSpec.ImageArtifactSource.StreamTo(&dest)
			if err != nil {
				logger.Error("failed-to-stream-image-artifact-source", err)
				return ImageMetadata{}, nil, "", err
			}
		}

		imageMetadataReader, err = imageSpec.ImageArtifactSource.StreamFile(ImageMetadataFile)
		if err != nil {
			logger.Error("failed-to-stream-metadata-file", err)
			return ImageMetadata{}, nil, "", err
		}
	}

	// 'image_resource:' in task
	if imageResource != nil {
		image := p.imageFactory.NewImage(
			logger.Session("image"),
			cancel,
			*imageResource,
			id,
			metadata,
			p.worker.Tags(),
			teamID,
			resourceTypes,
			p.worker,
			delegate,
			imageSpec.Privileged,
		)

		var imageParentVolume Volume
		var err error
		imageParentVolume, imageMetadataReader, version, err = image.Fetch()
		if err != nil {
			logger.Error("failed-to-fetch-image", err)
			return ImageMetadata{}, nil, "", err
		}

		imageVolume, err = p.volumeClient.FindOrCreateVolumeForContainer(
			logger.Session("create-cow-volume"),
			VolumeSpec{
				Strategy: ContainerRootFSStrategy{
					Parent: imageParentVolume,
				},
				Privileged: imageSpec.Privileged,
			},
			container,
			&dbng.Team{ID: teamID},
			"/",
		)
		if err != nil {
			logger.Error("failed-to-create-image-resource-volume", err)
			return ImageMetadata{}, nil, "", err
		}
	}

	if imageVolume != nil {
		metadata, err := loadMetadata(imageMetadataReader)
		if err != nil {
			return ImageMetadata{}, nil, "", err
		}

		imageURL := url.URL{
			Scheme: RawRootFSScheme,
			Path:   path.Join(imageVolume.Path(), "rootfs"),
		}

		return metadata, version, imageURL.String(), nil
	}

	// built-in resource type specified in step
	if imageSpec.ResourceType != "" {
		rootFSURL, resourceTypeVersion, err := p.getBuiltInResourceTypeImageForContainer(logger, container, imageSpec.ResourceType, teamID)
		if err != nil {
			return ImageMetadata{}, nil, "", err
		}

		return ImageMetadata{}, resourceTypeVersion, rootFSURL, nil
	}

	// 'image:' in task
	return ImageMetadata{}, nil, imageSpec.ImageURL, nil
}

func (p *containerProvider) getBuiltInResourceTypeImageForContainer(
	logger lager.Logger,
	container dbng.CreatingContainer,
	resourceTypeName string,
	teamID int,
) (string, atc.Version, error) {
	for _, t := range p.worker.ResourceTypes() {
		if t.Type == resourceTypeName {
			importVolumeSpec := VolumeSpec{
				Strategy: HostRootFSStrategy{
					Path:       t.Image,
					Version:    &t.Version,
					WorkerName: p.worker.Name(),
				},
				Privileged: true,
				Properties: VolumeProperties{},
			}

			importVolume, err := p.volumeClient.FindOrCreateVolumeForBaseResourceType(
				logger,
				importVolumeSpec,
				&dbng.Team{ID: teamID},
				resourceTypeName,
			)
			if err != nil {
				return "", atc.Version{}, err
			}

			cowVolume, err := p.volumeClient.FindOrCreateVolumeForContainer(
				logger,
				VolumeSpec{
					Strategy: ContainerRootFSStrategy{
						Parent: importVolume,
					},
					Privileged: true,
					Properties: VolumeProperties{},
				},
				container,
				&dbng.Team{ID: teamID},
				"/",
			)
			if err != nil {
				return "", atc.Version{}, err
			}

			rootFSURL := url.URL{
				Scheme: RawRootFSScheme,
				Path:   cowVolume.Path(),
			}

			return rootFSURL.String(), atc.Version{resourceTypeName: t.Version}, nil
		}
	}

	return "", atc.Version{}, ErrUnsupportedResourceType
}

func (p *containerProvider) maxContainerLifetime(metadata Metadata) time.Duration {
	if metadata.Type == db.ContainerTypeCheck {
		uptime := p.worker.Uptime()
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

func loadMetadata(tarReader io.ReadCloser) (ImageMetadata, error) {
	defer tarReader.Close()

	var imageMetadata ImageMetadata
	if err := json.NewDecoder(tarReader).Decode(&imageMetadata); err != nil {
		return ImageMetadata{}, MalformedMetadataError{
			UnmarshalError: err,
		}
	}

	return imageMetadata, nil
}

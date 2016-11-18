package worker

import (
	"encoding/json"
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

//go:generate counterfeiter . ContainerProviderFactory

type ContainerProviderFactory interface {
	ContainerProviderFor(Worker) ContainerProvider
}

type containerProviderFactory struct {
	gardenClient       garden.Client
	baggageclaimClient baggageclaim.Client
	volumeClient       VolumeClient
	imageFactory       ImageFactory
	dbContainerFactory DBContainerFactory
	dbVolumeFactory    dbng.VolumeFactory

	db GardenWorkerDB

	clock clock.Clock

	httpProxyURL  string
	httpsProxyURL string
	noProxy       string
}

func NewContainerProviderFactory(
	gardenClient garden.Client,
	baggageclaimClient baggageclaim.Client,
	volumeClient VolumeClient,
	imageFactory ImageFactory,
	dbContainerFactory DBContainerFactory,
	dbVolumeFactory dbng.VolumeFactory,
	db GardenWorkerDB,
	clock clock.Clock,
) ContainerProviderFactory {
	return &containerProviderFactory{
		gardenClient:       gardenClient,
		baggageclaimClient: baggageclaimClient,
		volumeClient:       volumeClient,
		imageFactory:       imageFactory,
		dbContainerFactory: dbContainerFactory,
		dbVolumeFactory:    dbVolumeFactory,
		db:                 db,
		clock:              clock,
	}
}

func (f *containerProviderFactory) ContainerProviderFor(
	worker Worker,
) ContainerProvider {
	return &containerProvider{
		gardenClient:       f.gardenClient,
		baggageclaimClient: f.baggageclaimClient,
		volumeClient:       f.volumeClient,
		imageFactory:       f.imageFactory,
		dbContainerFactory: f.dbContainerFactory,
		dbVolumeFactory:    f.dbVolumeFactory,
		db:                 f.db,
		clock:              f.clock,
		worker:             worker,
	}
}

//go:generate counterfeiter . ContainerProvider

type ContainerProvider interface {
	FindContainerByHandle(
		logger lager.Logger,
		handle string,
	) (Container, bool, error)

	FindOrCreateContainer(
		logger lager.Logger,
		cancel <-chan os.Signal,
		creatingContainer *dbng.CreatingContainer,
		delegate ImageFetchingDelegate,
		id Identifier,
		metadata Metadata,
		spec ContainerSpec,
		resourceTypes atc.ResourceTypes,
		outputPaths map[string]string,
	) (Container, error)
}

type containerProvider struct {
	gardenClient            garden.Client
	baggageclaimClient      baggageclaim.Client
	volumeClient            VolumeClient
	imageFactory            ImageFactory
	dbContainerFactory      DBContainerFactory
	dbVolumeFactory         dbng.VolumeFactory
	dbResourceCacheFactory  dbng.ResourceCacheFactory
	dbResourceConfigFactory dbng.ResourceConfigFactory

	db       GardenWorkerDB
	provider WorkerProvider

	clock clock.Clock

	worker        Worker
	httpProxyURL  string
	httpsProxyURL string
	noProxy       string
}

func (p *containerProvider) FindOrCreateContainer(
	logger lager.Logger,
	cancel <-chan os.Signal,
	creatingContainer *dbng.CreatingContainer,
	delegate ImageFetchingDelegate,
	id Identifier,
	metadata Metadata,
	spec ContainerSpec,
	resourceTypes atc.ResourceTypes,
	outputPaths map[string]string,
) (Container, error) {
	return p.createContainer(
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

	createdContainer, found, err := p.dbContainerFactory.FindContainer(handle)
	if err != nil {
		logger.Error("failed-to-lookup-in-db", err)
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	createdVolumes, err := p.dbVolumeFactory.FindVolumesForContainer(createdContainer.ID)
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
		p.clock,
		p.worker.Name(),
	)

	if err != nil {
		logger.Error("failed-to-construct-container", err)
		return nil, false, err
	}

	return container, true, nil
}

func (p *containerProvider) createContainer(
	logger lager.Logger,
	cancel <-chan os.Signal,
	creatingContainer *dbng.CreatingContainer,
	delegate ImageFetchingDelegate,
	id Identifier,
	metadata Metadata,
	spec ContainerSpec,
	resourceTypes atc.ResourceTypes,
	outputPaths map[string]string,
) (Container, error) {
	gardenContainer, err := p.createGardenContainer(
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
	if err != nil {
		return nil, err
	}

	createdContainer, err := p.dbContainerFactory.ContainerCreated(creatingContainer, gardenContainer.Handle()) // TODO: Set Created on Container
	if err != nil {
		logger.Error("failed-to-mark-container-as-created", err)
		return nil, err
	}

	createdVolumes, err := p.dbVolumeFactory.FindVolumesForContainer(createdContainer.ID)
	if err != nil {
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
		p.clock,
		p.worker.Name(),
	)
}

func (p *containerProvider) createGardenContainer(
	logger lager.Logger,
	cancel <-chan os.Signal,
	creatingContainer *dbng.CreatingContainer,
	delegate ImageFetchingDelegate,
	id Identifier,
	metadata Metadata,
	spec ContainerSpec,
	resourceTypes atc.ResourceTypes,
	outputPaths map[string]string,
) (garden.Container, error) {
	imageMetadata, resourceTypeVersion, imageURL, err := p.getImageForContainer(
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

	volumeMounts := []VolumeMount{}
	for name, outputPath := range outputPaths {
		outVolume, err := p.volumeClient.FindOrCreateVolumeForContainer(
			logger,
			VolumeSpec{
				Strategy:   OutputStrategy{Name: name},
				Privileged: bool(spec.ImageSpec.Privileged),
			},
			creatingContainer,
			&dbng.Team{ID: spec.TeamID},
			outputPath,
		)
		if err != nil {
			return nil, err
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
		cowVolume, err := p.volumeClient.FindOrCreateVolumeForContainer(
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
		if err != nil {
			return nil, err
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
	}

	gardenContainer, err := p.gardenClient.Create(gardenSpec)
	if err != nil {
		logger.Error("failed-to-create-container-in-garden", err)
		return nil, err
	}

	metadata.WorkerName = p.worker.Name()
	metadata.Handle = gardenContainer.Handle()
	metadata.User = gardenSpec.Properties["user"]

	id.ResourceTypeVersion = resourceTypeVersion

	_, err = p.db.UpdateContainerTTLToBeRemoved(
		db.Container{
			ContainerIdentifier: db.ContainerIdentifier(id),
			ContainerMetadata:   db.ContainerMetadata(metadata),
		},
		ContainerTTL,
		p.maxContainerLifetime(metadata),
	)
	if err != nil {
		logger.Error("failed-to-update-container-ttl", err)
		return nil, err
	}

	return gardenContainer, nil
}

func (p *containerProvider) getImageForContainer(
	logger lager.Logger,
	container *dbng.CreatingContainer,
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
	container *dbng.CreatingContainer,
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

// var gardenContainer garden.Container
// var gardenContainerFound bool
//
// creatingContainer, createdContainer, err := findContainerFunc()
// if err != nil {
//   logger.Error("failed-to-find-container-in-db", err)
//   return nil, err
// }
//
// if createdContainer != nil {
//   gardenContainer, gardenContainerFound, err = c.gardenClient.Lookup(
//     createdContainer.Handle(),
//   )
//   if err != nil {
//     logger.Error("failed-to-lookup-container-in-garden", err)
//     return nil, err
//   }
//
//   if !gardenContainerFound {
//     logger.Error("failed-to-lookup-container-in-garden", ErrCreatedContainerNotFound, lager.Data{"handle": createdContainer.Handle()})
//     return nil, ErrCreatedContainerNotFound
//   }
//
//   return NewVolume(gardenContainer, createdContainer), nil
// }
//
// if creatingVolume != nil {
//   gardenContainer, gardenContainerFound, err = c.gardenClient.Lookup(
//     creatingVolume.Handle(),
//   )
//   if err != nil {
//     logger.Error("failed-to-lookup-container-in-garden", err)
//     return nil, err
//   }
// } else {
//   creatingContainer, err = createContainerFunc()
//   if err != nil {
//     logger.Error("failed-to-create-container-in-garden", err)
//     return nil, err
//   }
// }
//
// lock, acquired, err := c.db.AcquireContainerCreatingLock(logger, creatingContainer.ID())
// if err != nil {
//   logger.Error("failed-to-acquire-container-creating-lock", err)
//   return nil, err
// }
//
// if !acquired {
//   c.clock.Sleep(creatingContainerRetryDelay)
//   return c.findOrCreateContainer(logger, containerSpec, findContainerFunc, createContainerFunc)
// }
//
// defer lock.Release()
//
// if !gardenContainerFound {
//   gardenContainer, err = worker.createGardenContainer()
//   if err != nil {
//     logger.Error("failed-to-create-container-in-garden", err)
//     return nil, err
//   }
// }
//
// createdContainer, err = worker.dbContainerFactory.ContainerCreated(creatingContainer, gardenContainer.Handle())
//
// if err != nil {
//   logger.Error("failed-to-mark-container-as-created", err)
//   return nil, err
// }
//
// metadata.WorkerName = worker.name
// metadata.Handle = gardenContainer.Handle()
// metadata.User = gardenSpec.Properties["user"]
//
// id.ResourceTypeVersion = resourceTypeVersion
//
// _, err = worker.db.UpdateContainerTTLToBeRemoved(
//   db.Container{
//     ContainerIdentifier: db.ContainerIdentifier(id),
//     ContainerMetadata:   db.ContainerMetadata(metadata),
//   },
//   ContainerTTL,
//   worker.maxContainerLifetime(metadata),
// )
// if err != nil {
//   logger.Error("failed-to-update-container-ttl", err)
//   return nil, err
// }
//
// createdVolumes, err := worker.dbVolumeFactory.FindVolumesForContainer(createdContainer.ID)
// if err != nil {
//   return nil, err
// }
//
// return newGardenWorkerContainer(
//   logger,
//   gardenContainer,
//   createdContainer,
//   createdVolumes,
//   worker.gardenClient,
//   worker.baggageclaimClient,
//   worker.db,
//   worker.clock,
//   worker.name,
// )

// func (worker *gardenWorker) createGardenContainer(
// 	logger lager.Logger,
// 	cancel <-chan os.Signal,
// 	creatingContainer *dbng.CreatingContainer,
// 	delegate ImageFetchingDelegate,
// 	id Identifier,
// 	metadata Metadata,
// 	spec ContainerSpec,
// 	resourceTypes atc.ResourceTypes,
// 	outputPaths map[string]string,
// ) (Container, error) {
// 	logger.Debug("[super-logs] running createContainer")
// 	defer logger.Debug("[super-logs] finished createContainer")
//
// 	imageMetadata, resourceTypeVersion, imageURL, err := worker.getImageForContainer(
// 		logger,
// 		creatingContainer,
// 		spec.ImageSpec,
// 		spec.TeamID,
// 		cancel,
// 		delegate,
// 		id,
// 		metadata,
// 		resourceTypes,
// 	)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	volumeMounts := []VolumeMount{}
// 	for name, outputPath := range outputPaths {
// 		outVolume, err := worker.volumeClient.FindOrCreateVolumeForContainer(
// 			logger,
// 			VolumeSpec{
// 				Strategy:   OutputStrategy{Name: name},
// 				Privileged: bool(spec.ImageSpec.Privileged),
// 			},
// 			creatingContainer,
// 			&dbng.Team{ID: spec.TeamID},
// 			outputPath,
// 		)
// 		if err != nil {
// 			return nil, err
// 		}
//
// 		volumeMounts = append(volumeMounts, VolumeMount{
// 			Volume:    outVolume,
// 			MountPath: outputPath,
// 		})
// 	}
//
// 	for _, mount := range spec.Mounts {
// 		volumeMounts = append(volumeMounts, mount)
// 	}
//
// 	for _, mount := range spec.Inputs {
// 		cowVolume, err := worker.volumeClient.FindOrCreateVolumeForContainer(
// 			logger,
// 			VolumeSpec{
// 				Strategy: ContainerRootFSStrategy{
// 					Parent: mount.Volume,
// 				},
// 				Privileged: spec.ImageSpec.Privileged,
// 			},
// 			creatingContainer,
// 			&dbng.Team{ID: spec.TeamID},
// 			mount.MountPath,
// 		)
// 		if err != nil {
// 			return nil, err
// 		}
//
// 		volumeMounts = append(volumeMounts, VolumeMount{
// 			Volume:    cowVolume,
// 			MountPath: mount.MountPath,
// 		})
// 	}
//
// 	bindMounts := []garden.BindMount{}
//
// 	volumeHandleMounts := map[string]string{}
// 	for _, mount := range volumeMounts {
// 		bindMounts = append(bindMounts, garden.BindMount{
// 			SrcPath: mount.Volume.Path(),
// 			DstPath: mount.MountPath,
// 			Mode:    garden.BindMountModeRW,
// 		})
// 		volumeHandleMounts[mount.Volume.Handle()] = mount.MountPath
// 	}
//
// 	gardenProperties := garden.Properties{userPropertyName: imageMetadata.User}
// 	if spec.User != "" {
// 		gardenProperties = garden.Properties{userPropertyName: spec.User}
// 	}
//
// 	if spec.Ephemeral {
// 		gardenProperties[ephemeralPropertyName] = "true"
// 	}
//
// 	env := append(imageMetadata.Env, spec.Env...)
//
// 	if worker.httpProxyURL != "" {
// 		env = append(env, fmt.Sprintf("http_proxy=%s", worker.httpProxyURL))
// 	}
//
// 	if worker.httpsProxyURL != "" {
// 		env = append(env, fmt.Sprintf("https_proxy=%s", worker.httpsProxyURL))
// 	}
//
// 	if worker.noProxy != "" {
// 		env = append(env, fmt.Sprintf("no_proxy=%s", worker.noProxy))
// 	}
//
// 	gardenSpec := garden.ContainerSpec{
// 		BindMounts: bindMounts,
// 		Privileged: spec.ImageSpec.Privileged,
// 		Properties: gardenProperties,
// 		RootFSPath: imageURL,
// 		Env:        env,
// 	}
//
// 	return worker.gardenClient.Create(gardenSpec)
// }

package image

import (
	"errors"
	"io"
	"net/url"
	"os"
	"path"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/worker"
)

var ErrUnsupportedResourceType = errors.New("unsupported resource type")

const RawRootFSScheme = "raw"

type imageFetcherFactory struct {
	imageFactory Factory
}

func NewImageFetcherFactory(
	imageFactory Factory,
) worker.ImageFetcherFactory {
	return &imageFetcherFactory{
		imageFactory: imageFactory,
	}
}

func (f *imageFetcherFactory) GetImageFetcher(
	logger lager.Logger,
	worker worker.Worker,
	volumeClient worker.VolumeClient,
	imageSpec worker.ImageSpec,
	teamID int,
	cancel <-chan os.Signal,
	delegate worker.ImageFetchingDelegate,
	id worker.Identifier,
	metadata worker.Metadata,
	resourceTypes atc.ResourceTypes,
) (worker.ImageFetcher, error) {
	if imageSpec.ImageArtifactSource != nil {
		artifactVolume, existsOnWorker, err := imageSpec.ImageArtifactSource.VolumeOn(worker)
		if err != nil {
			logger.Error("failed-to-check-if-volume-exists-on-worker", err)
			return nil, err
		}

		if existsOnWorker {
			return &imageProvidedByPreviousStepOnSameWorker{
				artifactVolume: artifactVolume,
				imageSpec:      imageSpec,
				teamID:         teamID,
				volumeClient:   volumeClient,
			}, nil
		}

		return &imageProvidedByPreviousStepOnDifferentWorker{
			imageSpec:    imageSpec,
			teamID:       teamID,
			volumeClient: volumeClient,
		}, nil
	}

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

	if imageResource != nil {
		image := f.imageFactory.NewImage(
			logger.Session("image"),
			cancel,
			*imageResource,
			id,
			metadata,
			worker.Tags(),
			teamID,
			resourceTypes,
			worker,
			delegate,
			imageSpec.Privileged,
		)

		imageParentVolume, imageMetadataReader, version, err := image.Fetch()
		if err != nil {
			logger.Error("failed-to-fetch-image", err)
			return nil, err
		}

		return &imageFromResource{
			imageParentVolume:   imageParentVolume,
			version:             version,
			imageMetadataReader: imageMetadataReader,
			imageSpec:           imageSpec,
			teamID:              teamID,
			volumeClient:        volumeClient,
		}, nil
	}

	if imageSpec.ResourceType != "" {
		return &imageFromBaseResourceType{
			worker:           worker,
			resourceTypeName: imageSpec.ResourceType,
			teamID:           teamID,
			volumeClient:     volumeClient,
		}, nil
	}

	return &imageInTask{
		url: imageSpec.ImageURL,
	}, nil
}

type imageProvidedByPreviousStepOnSameWorker struct {
	artifactVolume worker.Volume
	imageSpec      worker.ImageSpec
	teamID         int
	volumeClient   worker.VolumeClient
}

func (i *imageProvidedByPreviousStepOnSameWorker) FetchForContainer(
	logger lager.Logger,
	container dbng.CreatingContainer,
) (worker.FetchedImage, error) {
	imageVolume, err := i.volumeClient.FindOrCreateVolumeForContainer(
		logger,
		worker.VolumeSpec{
			Strategy: worker.ContainerRootFSStrategy{
				Parent: i.artifactVolume,
			},
			Privileged: i.imageSpec.Privileged,
		},
		container,
		&dbng.Team{ID: i.teamID},
		"/",
	)
	if err != nil {
		logger.Error("failed-to-create-image-artifact-cow-volume", err)
		return worker.FetchedImage{}, err
	}

	imageMetadataReader, err := i.imageSpec.ImageArtifactSource.StreamFile(ImageMetadataFile)
	if err != nil {
		logger.Error("failed-to-stream-metadata-file", err)
		return worker.FetchedImage{}, err
	}

	metadata, err := loadMetadata(imageMetadataReader)
	if err != nil {
		return worker.FetchedImage{}, err
	}

	imageURL := url.URL{
		Scheme: RawRootFSScheme,
		Path:   path.Join(imageVolume.Path(), "rootfs"),
	}

	return worker.FetchedImage{
		Metadata: metadata,
		URL:      imageURL.String(),
	}, nil
}

type imageProvidedByPreviousStepOnDifferentWorker struct {
	imageSpec    worker.ImageSpec
	teamID       int
	volumeClient worker.VolumeClient
}

func (i *imageProvidedByPreviousStepOnDifferentWorker) FetchForContainer(
	logger lager.Logger,
	container dbng.CreatingContainer,
) (worker.FetchedImage, error) {
	imageVolume, err := i.volumeClient.FindOrCreateVolumeForContainer(
		logger,
		worker.VolumeSpec{
			Strategy: worker.ImageArtifactReplicationStrategy{
				Name: string(i.imageSpec.ImageArtifactName),
			},
			Privileged: i.imageSpec.Privileged,
		},
		container,
		&dbng.Team{ID: i.teamID},
		"/",
	)
	if err != nil {
		logger.Error("failed-to-create-image-artifact-replicated-volume", err)
		return worker.FetchedImage{}, nil
	}

	dest := artifactDestination{
		destination: imageVolume,
	}

	err = i.imageSpec.ImageArtifactSource.StreamTo(&dest)
	if err != nil {
		logger.Error("failed-to-stream-image-artifact-source", err)
		return worker.FetchedImage{}, nil
	}

	imageMetadataReader, err := i.imageSpec.ImageArtifactSource.StreamFile(ImageMetadataFile)
	if err != nil {
		logger.Error("failed-to-stream-metadata-file", err)
		return worker.FetchedImage{}, err
	}

	metadata, err := loadMetadata(imageMetadataReader)
	if err != nil {
		return worker.FetchedImage{}, err
	}

	imageURL := url.URL{
		Scheme: RawRootFSScheme,
		Path:   path.Join(imageVolume.Path(), "rootfs"),
	}

	return worker.FetchedImage{
		Metadata: metadata,
		URL:      imageURL.String(),
	}, nil
}

type imageFromResource struct {
	imageParentVolume   worker.Volume
	version             atc.Version
	imageMetadataReader io.ReadCloser
	imageSpec           worker.ImageSpec
	teamID              int
	volumeClient        worker.VolumeClient
}

func (i *imageFromResource) FetchForContainer(
	logger lager.Logger,
	container dbng.CreatingContainer,
) (worker.FetchedImage, error) {
	imageVolume, err := i.volumeClient.FindOrCreateVolumeForContainer(
		logger.Session("create-cow-volume"),
		worker.VolumeSpec{
			Strategy: worker.ContainerRootFSStrategy{
				Parent: i.imageParentVolume,
			},
			Privileged: i.imageSpec.Privileged,
		},
		container,
		&dbng.Team{ID: i.teamID},
		"/",
	)
	if err != nil {
		logger.Error("failed-to-create-image-resource-volume", err)
		return worker.FetchedImage{}, err
	}

	metadata, err := loadMetadata(i.imageMetadataReader)
	if err != nil {
		return worker.FetchedImage{}, err
	}

	imageURL := url.URL{
		Scheme: RawRootFSScheme,
		Path:   path.Join(imageVolume.Path(), "rootfs"),
	}

	return worker.FetchedImage{
		Metadata: metadata,
		Version:  i.version,
		URL:      imageURL.String(),
	}, nil
}

type imageFromBaseResourceType struct {
	worker           worker.Worker
	resourceTypeName string
	teamID           int
	volumeClient     worker.VolumeClient
}

func (i *imageFromBaseResourceType) FetchForContainer(
	logger lager.Logger,
	container dbng.CreatingContainer,
) (worker.FetchedImage, error) {
	for _, t := range i.worker.ResourceTypes() {
		if t.Type == i.resourceTypeName {
			importVolumeSpec := worker.VolumeSpec{
				Strategy: worker.HostRootFSStrategy{
					Path:       t.Image,
					Version:    &t.Version,
					WorkerName: i.worker.Name(),
				},
				Privileged: true,
				Properties: worker.VolumeProperties{},
			}

			importVolume, err := i.volumeClient.FindOrCreateVolumeForBaseResourceType(
				logger,
				importVolumeSpec,
				&dbng.Team{ID: i.teamID},
				i.resourceTypeName,
			)
			if err != nil {
				return worker.FetchedImage{}, err
			}

			cowVolume, err := i.volumeClient.FindOrCreateVolumeForContainer(
				logger,
				worker.VolumeSpec{
					Strategy: worker.ContainerRootFSStrategy{
						Parent: importVolume,
					},
					Privileged: true,
					Properties: worker.VolumeProperties{},
				},
				container,
				&dbng.Team{ID: i.teamID},
				"/",
			)
			if err != nil {
				return worker.FetchedImage{}, err
			}

			rootFSURL := url.URL{
				Scheme: RawRootFSScheme,
				Path:   cowVolume.Path(),
			}

			return worker.FetchedImage{
				Metadata: worker.ImageMetadata{},
				Version:  atc.Version{i.resourceTypeName: t.Version},
				URL:      rootFSURL.String(),
			}, nil
		}
	}

	return worker.FetchedImage{}, ErrUnsupportedResourceType
}

type imageInTask struct {
	url string
}

func (i *imageInTask) FetchForContainer(
	logger lager.Logger,
	container dbng.CreatingContainer,
) (worker.FetchedImage, error) {
	return worker.FetchedImage{
		URL: i.url,
	}, nil
}

type artifactDestination struct {
	destination worker.Volume
}

func (wad *artifactDestination) StreamIn(path string, tarStream io.Reader) error {
	return wad.destination.StreamIn(path, tarStream)
}

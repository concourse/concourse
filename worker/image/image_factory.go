package image

import (
	"errors"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/worker"
)

var ErrUnsupportedResourceType = errors.New("unsupported resource type")

type imageFactory struct {
	imageResourceFetcherFactory ImageResourceFetcherFactory
}

func NewImageFactory(
	imageResourceFetcherFactory ImageResourceFetcherFactory,
) worker.ImageFactory {
	return &imageFactory{
		imageResourceFetcherFactory: imageResourceFetcherFactory,
	}
}

func (f *imageFactory) GetImage(
	logger lager.Logger,
	workerClient worker.Worker,
	volumeClient worker.VolumeClient,
	imageSpec worker.ImageSpec,
	teamID int,
	cancel <-chan os.Signal,
	delegate worker.ImageFetchingDelegate,
	resourceUser dbng.ResourceUser,
	resourceTypes atc.VersionedResourceTypes,
) (worker.Image, error) {
	if imageSpec.ImageArtifactSource != nil {
		artifactVolume, existsOnWorker, err := imageSpec.ImageArtifactSource.VolumeOn(workerClient)
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

	if imageSpec.ResourceType != "" {
		var foundCustom bool
		for _, resourceType := range resourceTypes {
			if resourceType.Name == imageSpec.ResourceType {
				foundCustom = true

				imageSpec = worker.ImageSpec{
					ImageResource: &atc.ImageResource{
						Source: resourceType.Source,
						Type:   resourceType.Type + "lol",
					},
					Privileged: resourceType.Privileged,
				}

				break
			}
		}

		if !foundCustom {
			return &imageFromBaseResourceType{
				worker:           workerClient,
				resourceTypeName: imageSpec.ResourceType,
				teamID:           teamID,
				volumeClient:     volumeClient,
			}, nil
		}
	}

	if imageSpec.ImageResource != nil {
		imageResourceFetcher := f.imageResourceFetcherFactory.ImageResourceFetcherFor(workerClient)
		imageParentVolume, imageMetadataReader, version, err := imageResourceFetcher.Fetch(
			logger.Session("image"),
			cancel,
			resourceUser,
			imageSpec.ImageResource.Type,
			imageSpec.ImageResource.Source,
			workerClient.Tags(),
			teamID,
			resourceTypes,
			delegate,
			imageSpec.Privileged,
		)
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

	return &imageFromRootfsURI{
		url: imageSpec.ImageURL,
	}, nil
}

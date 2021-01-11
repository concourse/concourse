package image

import (
	"context"
	"io"
	"net/url"
	"path"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/tracing"
)

const RawRootFSScheme = "raw"

const ImageMetadataFile = "metadata.json"

type imageProvidedByPreviousStepOnSameWorker struct {
	artifactVolume worker.Volume
	imageSpec      worker.ImageSpec
	teamID         int
	volumeClient   worker.VolumeClient
}

func (i *imageProvidedByPreviousStepOnSameWorker) FetchForContainer(
	ctx context.Context,
	logger lager.Logger,
	container db.CreatingContainer,
) (worker.FetchedImage, error) {
	imageVolume, err := i.volumeClient.FindOrCreateCOWVolumeForContainer(
		logger,
		worker.VolumeSpec{
			Strategy:   i.artifactVolume.COWStrategy(),
			Privileged: i.imageSpec.Privileged,
		},
		container,
		i.artifactVolume,
		i.teamID,
		"/",
	)
	if err != nil {
		logger.Error("failed-to-create-image-artifact-cow-volume", err)
		return worker.FetchedImage{}, err
	}

	imageMetadataReader, err := i.imageSpec.ImageArtifactSource.StreamFile(ctx, ImageMetadataFile)
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
		Metadata:   metadata,
		URL:        imageURL.String(),
		Privileged: i.imageSpec.Privileged,
	}, nil
}

type imageProvidedByPreviousStepOnDifferentWorker struct {
	imageSpec    worker.ImageSpec
	teamID       int
	volumeClient worker.VolumeClient
}

func (i *imageProvidedByPreviousStepOnDifferentWorker) FetchForContainer(
	ctx context.Context,
	logger lager.Logger,
	container db.CreatingContainer,
) (worker.FetchedImage, error) {
	ctx, span := tracing.StartSpan(ctx, "imageProvidedByPreviousStepOnDifferentWorker.FetchForContainer", tracing.Attrs{"container_id": container.Handle()})
	defer span.End()

	imageVolume, err := i.volumeClient.FindOrCreateVolumeForContainer(
		logger,
		worker.VolumeSpec{
			Strategy:   baggageclaim.EmptyStrategy{},
			Privileged: i.imageSpec.Privileged,
		},
		container,
		i.teamID,
		"/",
	)
	if err != nil {
		logger.Error("failed-to-create-image-artifact-replicated-volume", err)
		return worker.FetchedImage{}, err
	}

	dest := artifactDestination{
		destination: imageVolume,
	}

	err = i.imageSpec.ImageArtifactSource.StreamTo(ctx, &dest)
	if err != nil {
		logger.Error("failed-to-stream-image-artifact-source", err)
		return worker.FetchedImage{}, err
	}
	logger.Debug("streamed-non-local-image-volume")

	imageMetadataReader, err := i.imageSpec.ImageArtifactSource.StreamFile(ctx, ImageMetadataFile)
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
		Metadata:   metadata,
		URL:        imageURL.String(),
		Privileged: i.imageSpec.Privileged,
	}, nil
}

type imageFromBaseResourceType struct {
	worker           worker.Worker
	resourceTypeName string
	teamID           int
	volumeClient     worker.VolumeClient
}

func (i *imageFromBaseResourceType) FetchForContainer(
	ctx context.Context,
	logger lager.Logger,
	container db.CreatingContainer,
) (worker.FetchedImage, error) {
	for _, t := range i.worker.ResourceTypes() {
		if t.Type == i.resourceTypeName {
			importVolume, err := i.volumeClient.FindOrCreateVolumeForBaseResourceType(
				logger,
				worker.VolumeSpec{
					Strategy:   baggageclaim.ImportStrategy{Path: t.Image},
					Privileged: t.Privileged,
				},
				i.teamID,
				i.resourceTypeName,
			)
			if err != nil {
				return worker.FetchedImage{}, err
			}

			cowVolume, err := i.volumeClient.FindOrCreateCOWVolumeForContainer(
				logger,
				worker.VolumeSpec{
					Strategy:   importVolume.COWStrategy(),
					Privileged: t.Privileged,
				},
				container,
				importVolume,
				i.teamID,
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
				Metadata:   worker.ImageMetadata{},
				Version:    atc.Version{i.resourceTypeName: t.Version},
				URL:        rootFSURL.String(),
				Privileged: t.Privileged,
			}, nil
		}
	}

	return worker.FetchedImage{}, ErrUnsupportedResourceType
}

type imageFromRootfsURI struct {
	url string
}

func (i *imageFromRootfsURI) FetchForContainer(
	ctx context.Context,
	logger lager.Logger,
	container db.CreatingContainer,
) (worker.FetchedImage, error) {
	return worker.FetchedImage{
		URL: i.url,
	}, nil
}

type artifactDestination struct {
	destination worker.Volume
}

func (wad *artifactDestination) StreamIn(ctx context.Context, path string, encoding baggageclaim.Encoding, tarStream io.Reader) error {
	return wad.destination.StreamIn(ctx, path, encoding, tarStream)
}

func (wad *artifactDestination) GetStreamInP2pUrl(ctx context.Context, path string) (string, error) {
	return wad.destination.GetStreamInP2pUrl(ctx, path)
}

func (wad *artifactDestination) Volume() worker.Volume {
	return wad.destination
}
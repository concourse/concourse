package gardenruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/runtime"
)

const RawRootFSScheme = "raw"

const ImageMetadataFile = "metadata.json"

type FetchedImage struct {
	Metadata   ImageMetadata
	Version    atc.Version
	URL        string
	Privileged bool
}

type ImageMetadata struct {
	Env  []string `json:"env"`
	User string   `json:"user"`
}

func (worker *Worker) fetchImageForContainer(
	ctx context.Context,
	logger lager.Logger,
	imageSpec runtime.ImageSpec,
	teamID int,
	container db.CreatingContainer,
) (FetchedImage, error) {
	if imageSpec.ImageVolume != "" {
		volume, otherWorker, found, err := worker.pool.LocateVolume(logger, teamID, imageSpec.ImageVolume)
		if err != nil {
			logger.Error("failed-to-locate-artifact-volume", err)
			return FetchedImage{}, err
		}
		if !found {
			err = ErrImageArtifactVolumeNotFound
			logger.Error("image-artifact-volume-not-found", err)
			return FetchedImage{}, err
		}
		if worker.Name() == otherWorker.Name() {
			// it's on the same worker, so it will be a baggageclaim volume
			volumeOnWorker := volume.(Volume)
			return worker.imageProvidedByPreviousStepOnSameWorker(ctx, logger, imageSpec.Privileged, teamID, container, volumeOnWorker)
		} else {
			return worker.imageProvidedByPreviousStepOnDifferentWorker(ctx, logger, imageSpec.Privileged, teamID, container, volume, otherWorker.Name())
		}
	}

	if imageSpec.ResourceType != "" {
		for _, t := range worker.dbWorker.ResourceTypes() {
			if t.Type == imageSpec.ResourceType {
				return worker.imageFromBaseResourceType(ctx, logger, t, imageSpec.ResourceType, teamID, container)
			}
		}
		return FetchedImage{}, ErrUnsupportedResourceType
	}

	if imageSpec.ImageURL != "" {
		return FetchedImage{URL: imageSpec.ImageURL}, nil
	}

	return FetchedImage{}, ErrEmptyImageSpec
}

func (worker *Worker) imageProvidedByPreviousStepOnSameWorker(
	ctx context.Context,
	logger lager.Logger,
	privileged bool,
	teamID int,
	container db.CreatingContainer,
	artifactVolume Volume,
) (FetchedImage, error) {
	imageVolume, err := worker.findOrCreateCOWVolumeForContainer(
		logger,
		privileged,
		container,
		artifactVolume,
		teamID,
		"/",
	)
	if err != nil {
		logger.Error("failed-to-create-image-artifact-cow-volume", err)
		return FetchedImage{}, fmt.Errorf("create COW volume: %w", err)
	}

	imageMetadataReader, err := worker.streamer.StreamFile(ctx, artifactVolume, ImageMetadataFile)
	if err != nil {
		logger.Error("failed-to-stream-metadata-file", err)
		return FetchedImage{}, fmt.Errorf("stream metadata: %w", err)
	}

	metadata, err := loadMetadata(imageMetadataReader)
	if err != nil {
		return FetchedImage{}, fmt.Errorf("load metadata: %w", err)
	}

	imageURL := url.URL{
		Scheme: RawRootFSScheme,
		Path:   path.Join(imageVolume.Path(), "rootfs"),
	}

	return FetchedImage{
		Metadata:   metadata,
		URL:        imageURL.String(),
		Privileged: privileged,
	}, nil
}

func (worker *Worker) imageProvidedByPreviousStepOnDifferentWorker(
	ctx context.Context,
	logger lager.Logger,
	privileged bool,
	teamID int,
	container db.CreatingContainer,
	artifactVolume runtime.Volume,
	workerName string,
) (FetchedImage, error) {
	imageVolume, err := worker.findOrCreateVolumeForContainer(
		logger,
		runtime.VolumeSpec{
			Strategy:   baggageclaim.EmptyStrategy{},
			Privileged: privileged,
		},
		container,
		teamID,
		"/",
	)
	if err != nil {
		logger.Error("failed-to-create-image-artifact-replicated-volume", err)
		return FetchedImage{}, err
	}

	if err := worker.streamer.Stream(ctx, workerName, artifactVolume, imageVolume); err != nil {
		logger.Error("failed-to-stream-image-artifact", err)
		return FetchedImage{}, err
	}
	logger.Debug("streamed-non-local-image-volume")

	imageMetadataReader, err := worker.streamer.StreamFile(ctx, artifactVolume, ImageMetadataFile)
	if err != nil {
		logger.Error("failed-to-stream-metadata-file", err)
		return FetchedImage{}, err
	}

	metadata, err := loadMetadata(imageMetadataReader)
	if err != nil {
		return FetchedImage{}, err
	}

	imageURL := url.URL{
		Scheme: RawRootFSScheme,
		Path:   path.Join(imageVolume.Path(), "rootfs"),
	}

	return FetchedImage{
		Metadata:   metadata,
		URL:        imageURL.String(),
		Privileged: privileged,
	}, nil
}

func (worker *Worker) imageFromBaseResourceType(
	ctx context.Context,
	logger lager.Logger,
	resourceType atc.WorkerResourceType,
	resourceTypeName string,
	teamID int,
	container db.CreatingContainer,
) (FetchedImage, error) {
	importVolume, err := worker.findOrCreateVolumeForBaseResourceType(
		logger,
		runtime.VolumeSpec{
			Strategy:   baggageclaim.ImportStrategy{Path: resourceType.Image},
			Privileged: resourceType.Privileged,
		},
		teamID,
		resourceTypeName,
	)
	if err != nil {
		return FetchedImage{}, err
	}

	cowVolume, err := worker.findOrCreateCOWVolumeForContainer(
		logger,
		resourceType.Privileged,
		container,
		importVolume,
		teamID,
		"/",
	)
	if err != nil {
		return FetchedImage{}, err
	}

	rootFSURL := url.URL{
		Scheme: RawRootFSScheme,
		Path:   cowVolume.Path(),
	}

	return FetchedImage{
		Metadata:   ImageMetadata{},
		Version:    atc.Version{resourceTypeName: resourceType.Version},
		URL:        rootFSURL.String(),
		Privileged: resourceType.Privileged,
	}, nil
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

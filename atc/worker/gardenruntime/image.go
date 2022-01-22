package gardenruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/worker/baggageclaim"
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
	imageSpec runtime.ImageSpec,
	teamID int,
	container db.CreatingContainer,
	stderr io.Writer,
) (FetchedImage, error) {
	logger := lagerctx.FromContext(ctx)
	if imageSpec.ImageArtifact != nil {
		volume, ok, err := worker.findVolumeForArtifact(ctx, teamID, imageSpec.ImageArtifact, stderr)
		if err != nil {
			logger.Error("failed-to-locate-artifact-volume", err)
			return FetchedImage{}, err
		}
		if ok && volume.DBVolume().WorkerName() == worker.Name() {
			// it's on the same worker, so it will be a baggageclaim volume
			volumeOnWorker := volume.(Volume)
			fmt.Fprintf(stderr, "image found on same worker %s\n", worker.Name())
			return worker.imageProvidedByPreviousStepOnSameWorker(ctx, logger, imageSpec.Privileged, teamID, container, volumeOnWorker, stderr)
		} else {
			return worker.imageProvidedByPreviousStepOnDifferentWorker(ctx, imageSpec.Privileged, teamID, container, imageSpec.ImageArtifact, stderr)
		}
	}

	if imageSpec.ResourceType != "" {
		fmt.Fprintf(stderr, "before iterate worker.dbWorker.ResourceTypes()\n")
		for _, t := range worker.dbWorker.ResourceTypes() {
			if t.Type == imageSpec.ResourceType {
				fmt.Fprintf(stderr, "before imageFromBaseResourceType %s\n", imageSpec.ResourceType)
				return worker.imageFromBaseResourceType(ctx, t, imageSpec.ResourceType, teamID, container, stderr)
			}
		}
		return FetchedImage{}, ErrUnsupportedResourceType
	}

	return FetchedImage{URL: imageSpec.ImageURL}, nil
}

func (worker *Worker) imageProvidedByPreviousStepOnSameWorker(
	ctx context.Context,
	logger lager.Logger,
	privileged bool,
	teamID int,
	container db.CreatingContainer,
	artifactVolume Volume,
	stderr io.Writer,
) (FetchedImage, error) {
	fmt.Fprintf(stderr, "imageProvidedByPreviousStepOnSameWorker: before findOrCreateCOWVolumeForContainer\n")
	imageVolume, err := worker.findOrCreateCOWVolumeForContainer(
		ctx,
		privileged,
		container,
		artifactVolume,
		teamID,
		"/",
		stderr,
	)
	if err != nil {
		logger.Error("failed-to-create-image-artifact-cow-volume", err)
		return FetchedImage{}, fmt.Errorf("create COW volume: %w", err)
	}

	fmt.Fprintf(stderr, "imageProvidedByPreviousStepOnSameWorker: before StreamFile %s\n", ImageMetadataFile)
	imageMetadataReader, err := worker.streamer.StreamFile(ctx, artifactVolume, ImageMetadataFile)
	if err != nil {
		logger.Error("failed-to-stream-metadata-file", err)
		return FetchedImage{}, fmt.Errorf("stream metadata: %w", err)
	}

	fmt.Fprintf(stderr, "imageProvidedByPreviousStepOnSameWorker: before loadMetadata %s\n", ImageMetadataFile)
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
	privileged bool,
	teamID int,
	container db.CreatingContainer,
	artifact runtime.Artifact,
	stderr io.Writer,
) (FetchedImage, error) {
	logger := lagerctx.FromContext(ctx)
	streamedVolume, err := worker.findOrCreateVolumeForStreaming(
		ctx,
		privileged,
		container,
		teamID,
		"/",
		stderr,
	)
	if err != nil {
		logger.Error("failed-to-create-image-artifact-replicated-volume", err)
		return FetchedImage{}, err
	}

	if err := worker.streamer.Stream(ctx, artifact, streamedVolume); err != nil {
		logger.Error("failed-to-stream-image-artifact", err)
		return FetchedImage{}, err
	}
	logger.Debug("streamed-non-local-image-volume")

	imageVolume, err := worker.findOrCreateCOWVolumeForContainer(
		ctx,
		privileged,
		container,
		streamedVolume,
		teamID,
		"/",
		stderr,

	)
	if err != nil {
		logger.Error("failed-to-create-cow-volume-for-image", err)
		return FetchedImage{}, err
	}

	imageMetadataReader, err := worker.streamer.StreamFile(ctx, artifact, ImageMetadataFile)
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
	resourceType atc.WorkerResourceType,
	resourceTypeName string,
	teamID int,
	container db.CreatingContainer,
	stderr io.Writer,
) (FetchedImage, error) {
	fmt.Fprintf(stderr, "before findOrCreateVolumeForBaseResourceType\n")
	importVolume, err := worker.findOrCreateVolumeForBaseResourceType(
		ctx,
		baggageclaim.VolumeSpec{
			Strategy:   baggageclaim.ImportStrategy{Path: resourceType.Image},
			Privileged: resourceType.Privileged,
		},
		teamID,
		resourceTypeName,
		stderr,
	)
	if err != nil {
		return FetchedImage{}, err
	}

	fmt.Fprintf(stderr, "before imageFromBaseResourceType.findOrCreateCOWVolumeForContainer\n")
	cowVolume, err := worker.findOrCreateCOWVolumeForContainer(
		ctx,
		resourceType.Privileged,
		container,
		importVolume,
		teamID,
		"/",
		stderr,
	)
	if err != nil {
		return FetchedImage{}, err
	}
	fmt.Fprintf(stderr, "after imageFromBaseResourceType.findOrCreateCOWVolumeForContainer\n")

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

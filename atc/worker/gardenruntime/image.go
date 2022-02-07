package gardenruntime

import (
	"context"
	"encoding/json"
	"io"
	"net/url"
	"path"

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
	delegate runtime.BuildStepDelegate,
) (FetchedImage, error) {
	logger := lagerctx.FromContext(ctx)

	if imageSpec.ImageArtifact != nil {
		volume, err := worker.findOrStreamVolume(ctx, imageSpec.Privileged, teamID, container, imageSpec.ImageArtifact, "/", delegate)
		if err != nil {
			logger.Error("failed-to-find-or-stream-volume-for-image", err)
			return FetchedImage{}, err
		}

		imageVolume, err := worker.findOrCreateCOWVolumeForContainer(
			ctx,
			imageSpec.Privileged,
			container,
			volume,
			teamID,
			"/",
		)
		if err != nil {
			logger.Error("failed-to-create-cow-volume-for-image", err)
			return FetchedImage{}, err
		}

		imageMetadataReader, err := worker.streamer.StreamFile(ctx, imageSpec.ImageArtifact, ImageMetadataFile)
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
			Privileged: imageSpec.Privileged,
		}, nil
	}

	if imageSpec.ResourceType != "" {
		for _, t := range worker.dbWorker.ResourceTypes() {
			if t.Type == imageSpec.ResourceType {
				return worker.imageFromBaseResourceType(ctx, t, imageSpec.ResourceType, teamID, container)
			}
		}
		return FetchedImage{}, ErrUnsupportedResourceType
	}

	return FetchedImage{URL: imageSpec.ImageURL}, nil
}

func (worker *Worker) imageFromBaseResourceType(
	ctx context.Context,
	resourceType atc.WorkerResourceType,
	resourceTypeName string,
	teamID int,
	container db.CreatingContainer,
) (FetchedImage, error) {
	importVolume, err := worker.findOrCreateVolumeForBaseResourceType(
		ctx,
		baggageclaim.VolumeSpec{
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
		ctx,
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

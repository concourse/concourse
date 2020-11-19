package image

import (
	"errors"
	"fmt"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/worker"
)

var ErrUnsupportedResourceType = errors.New("unsupported resource type")

type imageFactory struct{}

func NewImageFactory() worker.ImageFactory {
	return &imageFactory{}
}

func (f *imageFactory) GetImage(
	logger lager.Logger,
	worker worker.Worker,
	volumeClient worker.VolumeClient,
	imageSpec worker.ImageSpec,
	teamID int,
) (worker.Image, error) {
	if imageSpec.ImageArtifactSource != nil {
		fmt.Fprintf(os.Stderr, "EVAN:imageFactory.GetImage go to imageProvidedByPreviousStep, imageSpec=%+v\n", imageSpec)
		artifactVolume, existsOnWorker, err := imageSpec.ImageArtifactSource.ExistsOn(logger, worker)
		if err != nil {
			logger.Error("failed-to-check-if-volume-exists-on-worker", err)
			return nil, err
		}

		if existsOnWorker {
			fmt.Fprintf(os.Stderr, "EVAN:imageFactory.GetImage go to imageProvidedByPreviousStep, on same worker %s\n", artifactVolume.WorkerName())
			return &imageProvidedByPreviousStepOnSameWorker{
				artifactVolume: artifactVolume,
				imageSpec:      imageSpec,
				teamID:         teamID,
				volumeClient:   volumeClient,
			}, nil
		}

		fmt.Fprintf(os.Stderr, "EVAN:imageFactory.GetImage go to imageProvidedByPreviousStep, on different worker\n")
		return &imageProvidedByPreviousStepOnDifferentWorker{
			imageSpec:    imageSpec,
			teamID:       teamID,
			volumeClient: volumeClient,
		}, nil
	}

	if imageSpec.ResourceType != "" {
		fmt.Fprintf(os.Stderr, "EVAN:imageFactory.GetImage go to imageFromBaseResourceType, worker=%s resourceTypeName=%s\n", worker.Name(), imageSpec.ResourceType)
		return &imageFromBaseResourceType{
			worker:           worker,
			resourceTypeName: imageSpec.ResourceType,
			teamID:           teamID,
			volumeClient:     volumeClient,
		}, nil
	}

	return &imageFromRootfsURI{
		url: imageSpec.ImageURL,
	}, nil
}

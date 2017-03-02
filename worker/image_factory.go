package worker

import (
	"io"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
)

//go:generate counterfeiter . ImageFactory

type ImageFactory interface {
	GetImage(
		logger lager.Logger,
		worker Worker,
		volumeClient VolumeClient,
		imageSpec ImageSpec,
		teamID int,
		signals <-chan os.Signal,
		delegate ImageFetchingDelegate,
		resourceUser dbng.ResourceUser,
		id Identifier,
		metadata Metadata,
		resourceTypes atc.ResourceTypes,
	) (Image, error)
}

type FetchedImage struct {
	Metadata ImageMetadata
	Version  atc.Version
	URL      string
}

//go:generate counterfeiter . Image

type Image interface {
	FetchForContainer(
		logger lager.Logger,
		container dbng.CreatingContainer,
	) (FetchedImage, error)
}

//go:generate counterfeiter . ImageFetchingDelegate

type ImageFetchingDelegate interface {
	Stderr() io.Writer
	ImageVersionDetermined(ResourceCacheIdentifier) error
}

type ImageMetadata struct {
	Env  []string `json:"env"`
	User string   `json:"user"`
}

type NoopImageFetchingDelegate struct{}

func (NoopImageFetchingDelegate) Stderr() io.Writer                                    { return ioutil.Discard }
func (NoopImageFetchingDelegate) ImageVersionDetermined(ResourceCacheIdentifier) error { return nil }

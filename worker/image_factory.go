package worker

import (
	"io"
	"io/ioutil"
	"os"

	"github.com/concourse/atc"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . ImageFactory

type ImageFactory interface {
	NewImage(
		logger lager.Logger,
		cancel <-chan os.Signal,
		imageResource atc.ImageResource,
		id Identifier,
		metadata Metadata,
		tags atc.Tags,
		teamID int,
		resourceTypes atc.ResourceTypes,
		workerClient Client,
		delegate ImageFetchingDelegate,
		privileged bool,
	) Image
}

//go:generate counterfeiter . Image

type Image interface {
	Fetch() (Volume, io.ReadCloser, atc.Version, error)
}

//go:generate counterfeiter . ImageFetchingDelegate

type ImageFetchingDelegate interface {
	Stderr() io.Writer
	ImageVersionDetermined(VolumeIdentifier) error
}

type ImageMetadata struct {
	Env  []string `json:"env"`
	User string   `json:"user"`
}

type NoopImageFetchingDelegate struct{}

func (NoopImageFetchingDelegate) Stderr() io.Writer                             { return ioutil.Discard }
func (NoopImageFetchingDelegate) ImageVersionDetermined(VolumeIdentifier) error { return nil }

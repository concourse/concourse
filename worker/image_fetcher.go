package worker

import (
	"io"
	"io/ioutil"
	"os"

	"github.com/concourse/atc"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . ImageFetcher

type ImageFetcher interface {
	FetchImage(
		logger lager.Logger,
		imageResource atc.ImageResource,
		cancel <-chan os.Signal,
		containerID Identifier,
		containerMetadata Metadata,
		delegate ImageFetchingDelegate,
		workerClient Client,
		tags atc.Tags,
		resourceTypes atc.ResourceTypes,
		privileged bool,
	) (Volume, io.ReadCloser, atc.Version, error)
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

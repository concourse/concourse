package worker

import (
	"io"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
)

//go:generate counterfeiter . ImageFactory

type ImageFactory interface {
	GetImage(
		logger lager.Logger,
		workerClient Worker,
		volumeClient VolumeClient,
		imageSpec ImageSpec,
		teamID int,
		delegate ImageFetchingDelegate,
		resourceTypes creds.VersionedResourceTypes,
	) (Image, error)
}

type FetchedImage struct {
	Metadata   ImageMetadata
	Version    atc.Version
	URL        string
	Privileged bool
}

//go:generate counterfeiter . Image

type Image interface {
	FetchForContainer(
		logger lager.Logger,
		cancel <-chan os.Signal,
		container db.CreatingContainer,
	) (FetchedImage, error)
}

//go:generate counterfeiter . ImageFetchingDelegate

type ImageFetchingDelegate interface {
	Stdout() io.Writer
	Stderr() io.Writer
	ImageVersionDetermined(*db.UsedResourceCache) error
}

type ImageMetadata struct {
	Env  []string `json:"env"`
	User string   `json:"user"`
}

type NoopImageFetchingDelegate struct{}

func (NoopImageFetchingDelegate) Stdout() io.Writer                                  { return ioutil.Discard }
func (NoopImageFetchingDelegate) Stderr() io.Writer                                  { return ioutil.Discard }
func (NoopImageFetchingDelegate) ImageVersionDetermined(*db.UsedResourceCache) error { return nil }

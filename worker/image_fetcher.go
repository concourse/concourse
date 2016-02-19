package worker

import (
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . ImageFetcher

type ImageFetcher interface {
	FetchImage(
		lager.Logger,
		atc.TaskImageConfig,
		<-chan os.Signal,
		Identifier,
		Metadata,
		ImageFetchingDelegate,
		Client,
		atc.ResourceTypes,
	) (Image, error)
}

//go:generate counterfeiter . ImageFetchingDelegate

type ImageFetchingDelegate interface {
	Stderr() io.Writer
	ImageVersionDetermined(db.VolumeIdentifier) error
}

//go:generate counterfeiter . Image

type Image interface {
	Volume() Volume
	Metadata() ImageMetadata
	Release(*time.Duration)
}

type ImageMetadata struct {
	Env  []string `json:"env"`
	User string   `json:"user"`
}

type NoopImageFetchingDelegate struct{}

func (NoopImageFetchingDelegate) Stderr() io.Writer                                { return ioutil.Discard }
func (NoopImageFetchingDelegate) ImageVersionDetermined(db.VolumeIdentifier) error { return nil }

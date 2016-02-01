package worker

import (
	"io"
	"os"

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
	// Env() []string
}

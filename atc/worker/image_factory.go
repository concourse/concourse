package worker

import (
	"context"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter . ImageFactory

type ImageFactory interface {
	GetImage(
		logger lager.Logger,
		worker Worker,
		volumeClient VolumeClient,
		imageSpec ImageSpec,
		teamID int,
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
		ctx context.Context,
		logger lager.Logger,
		container db.CreatingContainer,
	) (FetchedImage, error)
}

type ImageMetadata struct {
	Env  []string `json:"env"`
	User string   `json:"user"`
}

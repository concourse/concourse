package exec

import (
	"errors"
	"io"

	"github.com/concourse/atc"
	"github.com/tedsuo/ifrit"
)

var ErrFileNotFound = errors.New("file not found")

//go:generate counterfeiter . ArtifactSource
type ArtifactSource interface {
	ifrit.Runner

	StreamTo(ArtifactDestination) error
	StreamFile(path string) (io.ReadCloser, error)

	Release() error
}

//go:generate counterfeiter . ArtifactDestination
type ArtifactDestination interface {
	StreamIn(string, io.Reader) error
}

//go:generate counterfeiter . SuccessIndicator
type SuccessIndicator interface {
	Successful() bool
}

//go:generate counterfeiter . VersionIndicator
type VersionIndicator interface {
	Version() atc.Version
	Metadata() []atc.MetadataField
}

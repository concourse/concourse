package exec

import (
	"errors"
	"io"
	"os"

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

	Result(interface{}) bool
}

//go:generate counterfeiter . ArtifactDestination
type ArtifactDestination interface {
	StreamIn(string, io.Reader) error
}

type Success bool

type ExitStatus int

type VersionInfo struct {
	Version  atc.Version
	Metadata []atc.MetadataField
}

type NoopArtifactSource struct{}

func (NoopArtifactSource) Run(<-chan os.Signal, chan<- struct{}) error {
	return nil
}

func (NoopArtifactSource) Release() error { return nil }

func (NoopArtifactSource) StreamTo(ArtifactDestination) error {
	return nil
}

func (NoopArtifactSource) StreamFile(string) (io.ReadCloser, error) {
	return nil, ErrFileNotFound
}

func (NoopArtifactSource) Result(interface{}) bool {
	return false
}

type Identity struct{}

func (Identity) Using(source ArtifactSource) ArtifactSource {
	return source
}

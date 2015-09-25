package exec

import (
	"fmt"
	"io"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
	"github.com/concourse/baggageclaim"
	"github.com/tedsuo/ifrit"
)

type FileNotFoundError struct {
	Path string
}

func (err FileNotFoundError) Error() string {
	return fmt.Sprintf("file not found: %s", err.Path)
}

//go:generate counterfeiter . Step

type Step interface {
	ifrit.Runner

	Release()
	// Implementers of this method MUST not mutate the given pointer if they
	// are unable to respond (i.e. returning false from this function).
	Result(interface{}) bool
}

type SourceName string

//go:generate counterfeiter . ArtifactSource

type ArtifactSource interface {
	StreamTo(ArtifactDestination) error
	StreamFile(path string) (io.ReadCloser, error)

	VolumeOn(worker.Worker) (baggageclaim.Volume, bool, error)
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

type NoopStep struct{}

func (NoopStep) Run(<-chan os.Signal, chan<- struct{}) error {
	return nil
}

func (NoopStep) Release() {}

func (NoopStep) Result(interface{}) bool {
	return false
}

func (NoopStep) StreamTo(ArtifactDestination) error {
	return nil
}

func (NoopStep) StreamFile(path string) (io.ReadCloser, error) {
	return nil, FileNotFoundError{Path: path}
}

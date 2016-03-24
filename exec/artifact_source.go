package exec

import (
	"fmt"
	"io"

	"github.com/concourse/atc/worker"
)

//go:generate counterfeiter . ArtifactSource

// ArtifactSource represents data produced by the steps, that can be
// transferred to other steps.
type ArtifactSource interface {
	// StreamTo copies the data from the source to the destination. Note that
	// this potentially uses a lot of network transfer, for larger artifacts, as
	// the ATC will effectively act as a middleman.
	StreamTo(ArtifactDestination) error

	// StreamFile returns the contents of a single file in the artifact source.
	// This is used for loading a task's configuration at runtime.
	//
	// If the file cannot be found, FileNotFoundError should be returned.
	StreamFile(path string) (io.ReadCloser, error)

	// VolumeOn attempts to locate a volume equivalent to this source on the
	// given worker. If a volume can be found, it will be used directly. If not,
	// `StreamTo` will be used to copy the data to the destination instead.
	VolumeOn(worker.Worker) (worker.Volume, bool, error)
}

//go:generate counterfeiter . ArtifactDestination

// ArtifactDestination is the inverse of ArtifactSource. This interface allows
// the receiving end to determine the location of the data, e.g. based on a
// task's input configuration.
type ArtifactDestination interface {
	// StreamIn is called with a destination directory and the tar stream to
	// expand into the destination directory.
	StreamIn(string, io.Reader) error
}

// FileNotFoundError is the error to return from StreamFile when the given path
// does not exist.
type FileNotFoundError struct {
	Path string
}

// Error prints a helpful message including the file path. The user will see
// this message if e.g. their task config path does not exist.
func (err FileNotFoundError) Error() string {
	return fmt.Sprintf("file not found: %s", err.Path)
}

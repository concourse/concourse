package worker

import (
	"archive/tar"
	"context"
	"io"

	"code.cloudfoundry.org/lager"
	"github.com/klauspost/compress/zstd"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/hashicorp/go-multierror"
)

//go:generate counterfeiter . ArtifactSource

type ArtifactSource interface {
	// ExistsOn attempts to locate a volume equivalent to this source on the
	// given worker. If a volume can be found, it will be used directly. If not,
	// `StreamTo` will be used to copy the data to the destination instead.
	ExistsOn(lager.Logger, Worker) (Volume, bool, error)
}

//go:generate counterfeiter . StreamableArtifactSource

// Source represents data produced by the steps, that can be transferred to
// other steps.
type StreamableArtifactSource interface {
	ArtifactSource
	// StreamTo copies the data from the source to the destination. Note that
	// this potentially uses a lot of network transfer, for larger artifacts, as
	// the ATC will effectively act as a middleman.
	StreamTo(context.Context, lager.Logger, ArtifactDestination) error

	// StreamFile returns the contents of a single file in the artifact source.
	// This is used for loading a task's configuration at runtime.
	//
	// If the file cannot be found, FileNotFoundError should be returned.
	StreamFile(context.Context, lager.Logger, string) (io.ReadCloser, error)
}

type artifactSource struct {
	artifact runtime.Artifact
	volume   Volume
}

func NewStreamableArtifactSource(artifact runtime.Artifact, volume Volume) StreamableArtifactSource {
	return &artifactSource{artifact: artifact, volume: volume}
}

// TODO: figure out if we want logging before and after streams, I remove logger from private methods
func (source *artifactSource) StreamTo(
	ctx context.Context,
	logger lager.Logger,
	destination ArtifactDestination,
) error {
	out, err := source.volume.StreamOut(ctx, ".")
	if err != nil {
		return err
	}

	defer out.Close()

	err = destination.StreamIn(ctx, ".", out)
	if err != nil {
		return err
	}
	return nil
}

// TODO: figure out if we want logging before and after streams, I remove logger from private methods
func (source *artifactSource) StreamFile(
	ctx context.Context,
	logger lager.Logger,
	filepath string,
) (io.ReadCloser, error) {
	out, err := source.volume.StreamOut(ctx, filepath)
	if err != nil {
		return nil, err
	}

	zstdReader, err := zstd.NewReader(out)
	if err != nil {
		return nil, err
	}
	tarReader := tar.NewReader(zstdReader)

	_, err = tarReader.Next()
	if err != nil {
		return nil, runtime.FileNotFoundError{Path: filepath}
	}

	return fileReadMultiCloser{
		reader: tarReader,
		closers: []io.Closer{
			out,
			CloseWithError{zstdReader},
		},
	}, nil
}

func (source *artifactSource) ExistsOn(logger lager.Logger, worker Worker) (Volume, bool, error) {
	return worker.LookupVolume(logger, source.artifact.ID())
}

type cacheArtifactSource struct {
	runtime.CacheArtifact
}

func NewCacheArtifactSource(artifact runtime.CacheArtifact) ArtifactSource {
	return &cacheArtifactSource{artifact}
}

func (source *cacheArtifactSource) ExistsOn(logger lager.Logger, worker Worker) (Volume, bool, error) {
	return worker.FindVolumeForTaskCache(logger, source.TeamID, source.JobID, source.StepName, source.Path)
}

type CloseWithError struct {
	NoErringCloser interface {
		Close()
	}
}

// I know this is bad
func (cwe CloseWithError) Close() error {
	cwe.NoErringCloser.Close()
	return nil
}

type fileReadMultiCloser struct {
	reader  io.Reader
	closers []io.Closer
}

func (frc fileReadMultiCloser) Read(p []byte) (n int, err error) {
	return frc.reader.Read(p)
}

func (frc fileReadMultiCloser) Close() error {
	var closeErrors error

	for _, closer := range frc.closers {
		err := closer.Close()
		if err != nil {
			closeErrors = multierror.Append(closeErrors, err)
		}
	}

	return closeErrors
}

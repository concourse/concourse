package worker

import (
	"archive/tar"
	"context"
	"io"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/compression"
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
	StreamFile(context.Context, lager.Logger, string) (io.ReadCloser, error)
}

type artifactSource struct {
	artifact    runtime.Artifact
	volume      Volume
	compression compression.Compression
}

func NewStreamableArtifactSource(
	artifact runtime.Artifact,
	volume Volume,
	compression compression.Compression,
) StreamableArtifactSource {
	return &artifactSource{
		artifact:    artifact,
		volume:      volume,
		compression: compression,
	}
}

// TODO: figure out if we want logging before and after streams, I remove logger from private methods
func (source *artifactSource) StreamTo(
	ctx context.Context,
	logger lager.Logger,
	destination ArtifactDestination,
) error {
	out, err := source.volume.StreamOut(ctx, ".", source.compression.Encoding())
	if err != nil {
		return err
	}

	defer out.Close()

	err = destination.StreamIn(ctx, ".", source.compression.Encoding(), out)
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
	out, err := source.volume.StreamOut(ctx, filepath, source.compression.Encoding())
	if err != nil {
		return nil, err
	}

	compressionReader, err := source.compression.NewReader(out)
	if err != nil {
		return nil, err
	}
	tarReader := tar.NewReader(compressionReader)

	_, err = tarReader.Next()
	if err != nil {
		return nil, err
	}

	return fileReadMultiCloser{
		reader: tarReader,
		closers: []io.Closer{
			out,
			compressionReader,
		},
	}, nil
}

// Returns volume if it belongs to the worker
//  otherwise, if the volume has a Resource Cache
//  it checks the worker for a local volume corresponding to the Resource Cache.
//  Note: The returned volume may have a different handle than the ArtifactSource's inner volume handle.
func (source *artifactSource) ExistsOn(logger lager.Logger, worker Worker) (Volume, bool, error) {
	if source.volume.WorkerName() == worker.Name() {
		return source.volume, true, nil
	}

	resourceCache, found, err := worker.FindResourceCacheForVolume(source.volume)
	if err != nil {
		return nil, false, err
	}
	if found {
		return worker.FindVolumeForResourceCache(logger, resourceCache)
	} else {
		return nil, false, nil
	}

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

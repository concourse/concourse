package worker

import (
	"archive/tar"
	"context"
	"io"

	"github.com/DataDog/zstd"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/hashicorp/go-multierror"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . ArtifactSource

// Source represents data produced by the steps, that can be transferred to
// other steps.
type ArtifactSource interface {
	// StreamTo copies the data from the source to the destination. Note that
	// this potentially uses a lot of network transfer, for larger artifacts, as
	// the ATC will effectively act as a middleman.
	StreamTo(context.Context, lager.Logger, ArtifactDestination) error

	// StreamFile returns the contents of a single file in the artifact source.
	// This is used for loading a task's configuration at runtime.
	//
	// If the file cannot be found, FileNotFoundError should be returned.
	StreamFile(context.Context, lager.Logger, string) (io.ReadCloser, error)

	// VolumeOn attempts to locate a volume equivalent to this source on the
	// given worker. If a volume can be found, it will be used directly. If not,
	// `StreamTo` will be used to copy the data to the destination instead.
	VolumeOn(lager.Logger, Worker) (Volume, bool, error)
}

type artifactSource struct {
	artifact runtime.Artifact
	volume   Volume
}

func NewArtifactSource(artifact runtime.Artifact, volume Volume) ArtifactSource {
	return &artifactSource{artifact: artifact, volume: volume}
}

//TODO: do we want these to be implemented for task cache source?
// It was not used before
func (source *artifactSource) StreamTo(ctx context.Context, logger lager.Logger, dest ArtifactDestination) error {
	return streamToHelper(ctx, source.volume, logger, dest)
}

func (source *artifactSource) StreamFile(ctx context.Context, logger lager.Logger, path string) (io.ReadCloser, error) {
	return streamFileHelper(ctx, source.volume, logger, path)
}

func (source *artifactSource) VolumeOn(logger lager.Logger, worker Worker) (Volume, bool, error) {
	if taskCacheArtifact, ok := source.artifact.(runtime.TaskCacheArtifact); ok {
		return worker.FindVolumeForTaskCache(logger, taskCacheArtifact.TeamID, taskCacheArtifact.JobID, taskCacheArtifact.StepName, taskCacheArtifact.Path)
	}

	return worker.LookupVolume(logger, source.artifact.ID())
}

func streamToHelper(
	ctx context.Context,
	s interface {
		StreamOut(context.Context, string) (io.ReadCloser, error)
	},
	logger lager.Logger,
	destination ArtifactDestination,
) error {
	// TODO: doing this for the taskCache case
	if s == nil {
		return nil
	}
	logger.Debug("start")

	defer logger.Debug("end")

	out, err := s.StreamOut(ctx, ".")
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	defer out.Close()

	err = destination.StreamIn(ctx, ".", out)
	if err != nil {
		logger.Error("failed", err)
		return err
	}
	return nil
}

func streamFileHelper(
	ctx context.Context,
	s interface {
		StreamOut(context.Context, string) (io.ReadCloser, error)
	},
	logger lager.Logger,
	path string,
) (io.ReadCloser, error) {
	out, err := s.StreamOut(ctx, path)
	if err != nil {
		return nil, err
	}

	zstdReader := zstd.NewReader(out)
	tarReader := tar.NewReader(zstdReader)

	_, err = tarReader.Next()
	if err != nil {
		return nil, runtime.FileNotFoundError{Path: path}
	}

	return fileReadMultiCloser{
		reader: tarReader,
		closers: []io.Closer{
			out,
			zstdReader,
		},
	}, nil
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

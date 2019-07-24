package worker

import (
	"archive/tar"
	"context"
	"errors"
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

type getArtifactSource struct {
	artifact runtime.GetArtifact
	volume   Volume
}

func (source *getArtifactSource) StreamTo(ctx context.Context, logger lager.Logger, dest ArtifactDestination) error {
	return streamToHelper(ctx, source.volume, logger, dest)
}

func (source *getArtifactSource) StreamFile(ctx context.Context, logger lager.Logger, path string) (io.ReadCloser, error) {
	return streamFileHelper(ctx, source.volume, logger, path)
}
func (source *getArtifactSource) VolumeOn(logger lager.Logger, worker Worker) (Volume, bool, error) {
	return worker.LookupVolume(logger, source.artifact.ID())
}

type taskArtifactSource struct {
	artifact runtime.Artifact
	volume   Volume
}

func (source *taskArtifactSource) StreamTo(ctx context.Context, logger lager.Logger, dest ArtifactDestination) error {
	return streamToHelper(ctx, source.volume, logger, dest)
}

func (source *taskArtifactSource) StreamFile(ctx context.Context, logger lager.Logger, path string) (io.ReadCloser, error) {
	return streamFileHelper(ctx, source.volume, logger, path)
}

func (source *taskArtifactSource) VolumeOn(logger lager.Logger, worker Worker) (Volume, bool, error) {
	return worker.LookupVolume(logger, source.artifact.ID())
}

type taskCacheArtifactSource struct {
	artifact runtime.Artifact
	volume   Volume
}

//TODO: do we want these to be implemented?
// It was not used before
func (source *taskCacheArtifactSource) StreamTo(ctx context.Context, logger lager.Logger, dest ArtifactDestination) error {
	return streamToHelper(ctx, source.volume, logger, dest)
}

func (source *taskCacheArtifactSource) StreamFile(ctx context.Context, logger lager.Logger, path string) (io.ReadCloser, error) {
	return streamFileHelper(ctx, source.volume, logger, path)
}

func (source *taskCacheArtifactSource) VolumeOn(logger lager.Logger, worker Worker) (Volume, bool, error) {

	if taskCacheArt, ok := source.artifact.(runtime.TaskCacheArtifact); ok {
		return worker.FindVolumeForTaskCache(logger, taskCacheArt.TeamID, taskCacheArt.JobID, taskCacheArt.StepName, taskCacheArt.Path)
	} else {
		logger.Fatal("incorrect-artifact-type-for-TaskCacheArtifactSource", errors.New("ded"), nil)
		panic(source.artifact)
	}

}

func streamToHelper(
	ctx context.Context,
	s interface {
		StreamOut(context.Context, string) (io.ReadCloser, error)
	},
	logger lager.Logger,
	destination ArtifactDestination,
) error {
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

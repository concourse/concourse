package worker

import (
	"archive/tar"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager/lagerctx"
	"context"
	"fmt"
	"io"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/tracing"
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
	StreamTo(context.Context, ArtifactDestination) error

	// StreamFile returns the contents of a single file in the artifact source.
	// This is used for loading a task's configuration at runtime.
	StreamFile(context.Context, string) (io.ReadCloser, error)
}

type artifactSource struct {
	artifact            runtime.Artifact
	volume              Volume
	compression         compression.Compression
	enabledP2pStreaming bool
	p2pStreamingTimeout time.Duration
	clock               clock.Clock
}

func NewStreamableArtifactSource(
	artifact runtime.Artifact,
	volume Volume,
	compression compression.Compression,
	enabledP2pStreaming bool,
	p2pStreamingTimeout time.Duration,
	clk clock.Clock,
) StreamableArtifactSource {
	if clk == nil {
		clk = clock.NewClock()
	}
	return &artifactSource{
		artifact:            artifact,
		volume:              volume,
		compression:         compression,
		enabledP2pStreaming: enabledP2pStreaming,
		p2pStreamingTimeout: p2pStreamingTimeout,
		clock:               clk,
	}
}

func (source *artifactSource) StreamTo(
	ctx context.Context,
	destination ArtifactDestination,
) error {
	logger := lagerctx.FromContext(ctx).Session("stream-to")
	logger.Info("start")
	defer logger.Info("end")

	ctx, span := tracing.StartSpan(ctx, "artifactSource.StreamTo", nil)
	defer span.End()

	var err error
	if !source.enabledP2pStreaming {
		err = source.streamTo(ctx, destination)
	} else {
		err = source.p2pStreamTo(ctx, destination)
	}

	// Inc counter if no error occurred.
	if err == nil {
		metric.Metrics.VolumesStreamed.Inc()
	}

	return err
}

func (source *artifactSource) streamTo(
	ctx context.Context,
	destination ArtifactDestination,
) error {
	_, outSpan := tracing.StartSpan(ctx, "volume.StreamOut", tracing.Attrs{
		"origin-volume": source.volume.Handle(),
		"origin-worker": source.volume.WorkerName(),
	})
	defer outSpan.End()
	out, err := source.volume.StreamOut(ctx, ".", source.compression.Encoding())

	if err != nil {
		tracing.End(outSpan, err)
		return err
	}

	defer out.Close()

	return destination.StreamIn(ctx, ".", source.compression.Encoding(), out)
}

func (source *artifactSource) p2pStreamTo(
	ctx context.Context,
	destination ArtifactDestination,
) error {
	ch := make(chan error, 1)
	streamInUrl := ""
	go func(ctx context.Context, ch chan<- error) {
		var err error
		streamInUrl, err = destination.GetStreamInP2pUrl(ctx, ".")
		if err != nil {
			ch <- err
			return
		}
		ch <- source.volume.StreamP2pOut(ctx, ".", streamInUrl, source.compression.Encoding())
	}(ctx, ch)

	timer := source.clock.NewTimer(source.p2pStreamingTimeout)
	select {
	case <-timer.C():
		return fmt.Errorf("p2p stream to %s timeout", streamInUrl)
	case err := <-ch:
		return err
	}
}

func (source *artifactSource) StreamFile(
	ctx context.Context,
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

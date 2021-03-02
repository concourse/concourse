package worker

import (
	"archive/tar"
	"context"
	"fmt"
	"github.com/concourse/concourse/atc/db"
	"io"
	"time"

	"code.cloudfoundry.org/lager/lagerctx"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/tracing"
	"github.com/hashicorp/go-multierror"
)

//go:generate counterfeiter . ArtifactSourcer

type ArtifactSourcer interface {
	SourceInputsAndCaches(logger lager.Logger, teamID int, inputMap map[string]runtime.Artifact) ([]InputSource, error)
	SourceImage(logger lager.Logger, imageArtifact runtime.Artifact) (StreamableArtifactSource, error)
}

type artifactSourcer struct {
	compression          compression.Compression
	volumeFinder         VolumeFinder
	enableP2PStreaming   bool
	p2pStreamingTimeout  time.Duration
	resourceCacheFactory db.ResourceCacheFactory
}

func NewArtifactSourcer(
	compression compression.Compression,
	volumeFinder VolumeFinder,
	enableP2PStreaming bool,
	p2pStreamingTimeout time.Duration,
	resourceCacheFactory db.ResourceCacheFactory,
) ArtifactSourcer {
	return artifactSourcer{
		compression:          compression,
		volumeFinder:         volumeFinder,
		enableP2PStreaming:   enableP2PStreaming,
		p2pStreamingTimeout:  p2pStreamingTimeout,
		resourceCacheFactory: resourceCacheFactory,
	}
}

func (w artifactSourcer) SourceInputsAndCaches(logger lager.Logger, teamID int, inputMap map[string]runtime.Artifact) ([]InputSource, error) {
	var inputs []InputSource
	for path, artifact := range inputMap {
		if cache, ok := artifact.(*runtime.CacheArtifact); ok {
			// task caches may not have a volume, it will be discovered on
			// the worker later. We do not stream task caches
			source := NewCacheArtifactSource(*cache)
			inputs = append(inputs, inputSource{source, path})
		} else {
			artifactVolume, found, err := w.volumeFinder.FindVolume(logger, teamID, artifact.ID())
			if err != nil {
				return nil, err
			}
			if !found {
				return nil, fmt.Errorf("volume not found for artifact id %v type %T", artifact.ID(), artifact)
			}

			source := NewStreamableArtifactSource(artifact, artifactVolume, w.compression, w.enableP2PStreaming, w.p2pStreamingTimeout, w.resourceCacheFactory)
			inputs = append(inputs, inputSource{source, path})
		}
	}

	return inputs, nil
}

func (w artifactSourcer) SourceImage(logger lager.Logger, imageArtifact runtime.Artifact) (StreamableArtifactSource, error) {
	artifactVolume, found, err := w.volumeFinder.FindVolume(logger, 0, imageArtifact.ID())
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("volume not found for artifact id %v type %T", imageArtifact.ID(), imageArtifact)
	}

	return NewStreamableArtifactSource(imageArtifact, artifactVolume, w.compression, w.enableP2PStreaming, w.p2pStreamingTimeout, w.resourceCacheFactory), nil
}

//go:generate counterfeiter . ArtifactSource

type ArtifactSource interface {
	// ExistsOn attempts to locate a volume equivalent to this source on the
	// given worker. If a volume can be found, it will be used directly. If not,
	// `StreamTo` will be used to copy the data to the destination instead.
	ExistsOn(lager.Logger, Worker) (Volume, bool, error)

	// TODO: EVAN, for debug, delete it before merge the PR
	Handle() string
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
	artifact             runtime.Artifact
	volume               Volume
	compression          compression.Compression
	enabledP2pStreaming  bool
	p2pStreamingTimeout  time.Duration
	resourceCacheFactory db.ResourceCacheFactory
}

func NewStreamableArtifactSource(
	artifact runtime.Artifact,
	volume Volume,
	compression compression.Compression,
	enabledP2pStreaming bool,
	p2pStreamingTimeout time.Duration,
	resourceCacheFactory db.ResourceCacheFactory,
) StreamableArtifactSource {
	return &artifactSource{
		artifact:             artifact,
		volume:               volume,
		compression:          compression,
		enabledP2pStreaming:  enabledP2pStreaming,
		p2pStreamingTimeout:  p2pStreamingTimeout,
		resourceCacheFactory: resourceCacheFactory,
	}
}

// TODO: EVAN, delete it before merge the PR
func (source *artifactSource) Handle() string {
	return source.volume.Handle()
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

	if err != nil {
		return err
	}

	// Inc counter if no error occurred.
	metric.Metrics.VolumesStreamed.Inc()

	usedResourceCache, found, err := source.resourceCacheFactory.FindResourceCacheByID(source.volume.GetResourceCacheID())
	if err != nil {
		logger.Error("artifactSource-StreamTo-failed-to-find-resource-cache", err)
		return nil
	}
	if !found {
		logger.Info("artifactSource.StreamTo-not-find-resource-cache, this should not happen",
			lager.Data{"rcId": source.volume.GetResourceCacheID(), "volumeHandle": source.volume.Handle()})
		return nil
	}

	err = destination.InitializeResourceCache(usedResourceCache)
	if err != nil {
		logger.Error("artifactSource.StreamTo-failed-init-resource-cache-on-dest-worker", err)
		return nil
	}

	metric.Metrics.StreamedResourceCaches.Inc()

	return nil
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
	getCtx, getCancel := context.WithTimeout(ctx, 5*time.Second)
	defer getCancel()
	streamInUrl, err := destination.GetStreamInP2pUrl(getCtx, ".")
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return context.Canceled
	default:
	}

	_, outSpan := tracing.StartSpan(ctx, "volume.P2pStreamOut", tracing.Attrs{
		"origin-volume": source.volume.Handle(),
		"origin-worker": source.volume.WorkerName(),
		"stream-in-url": streamInUrl,
	})
	defer outSpan.End()

	putCtx, putCancel := context.WithTimeout(ctx, source.p2pStreamingTimeout)
	defer putCancel()
	return source.volume.StreamP2pOut(putCtx, ".", streamInUrl, source.compression.Encoding())
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

// TODO: EVAN, delete it before merge the PR
func (source *cacheArtifactSource) Handle() string {
	return "cacheArtifactSource-na"
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

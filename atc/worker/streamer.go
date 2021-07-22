package worker

import (
	"archive/tar"
	"context"
	"io"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/tracing"
	"github.com/hashicorp/go-multierror"
)

type Streamer struct {
	compression compression.Compression
	p2p         P2PConfig

	resourceCacheFactory db.ResourceCacheFactory
}

type P2PConfig struct {
	Enabled bool
	Timeout time.Duration
}

func NewStreamer(cacheFactory db.ResourceCacheFactory, compression compression.Compression, p2p P2PConfig) Streamer {
	return Streamer{
		resourceCacheFactory: cacheFactory,
		compression:          compression,
		p2p:                  p2p,
	}
}

func (s Streamer) Stream(ctx context.Context, src runtime.Artifact, dst runtime.Volume) error {
	loggerData := lager.Data{
		"to":        dst.DBVolume().WorkerName(),
		"to-handle": dst.Handle(),
	}
	srcVolume, isSrcVolume := src.(runtime.Volume)
	if isSrcVolume {
		loggerData["from"] = srcVolume.DBVolume().WorkerName()
		loggerData["from-handle"] = srcVolume.Handle()
	}
	logger := lagerctx.FromContext(ctx).Session("stream", loggerData)
	logger.Info("start")
	defer logger.Info("end")

	err := s.stream(ctx, src, dst)
	if err != nil {
		return err
	}

	if !isSrcVolume {
		return nil
	}

	metric.Metrics.VolumesStreamed.Inc()

	resourceCacheID := srcVolume.DBVolume().GetResourceCacheID()
	if atc.EnableCacheStreamedVolumes && resourceCacheID != 0 {
		logger.Debug("initialize-streamed-resource-cache", lager.Data{"resource-cache-id": resourceCacheID})
		usedResourceCache, found, err := s.resourceCacheFactory.FindResourceCacheByID(resourceCacheID)
		if err != nil {
			logger.Error("stream-to-failed-to-find-resource-cache", err)
			return err
		}
		if !found {
			logger.Info("stream-resource-cache-not-found-should-not-happen", lager.Data{
				"resource-cache-id": resourceCacheID,
				"volume":            srcVolume.Handle(),
			})
			return StreamingResourceCacheNotFoundError{
				Handle:          srcVolume.Handle(),
				ResourceCacheID: resourceCacheID,
			}
		}

		err = dst.InitializeStreamedResourceCache(logger, usedResourceCache, srcVolume.DBVolume().WorkerName())
		if err != nil {
			logger.Error("failed-to-init-resource-cache-on-dest-worker", err)
			return err
		}

		metric.Metrics.StreamedResourceCaches.Inc()
	}
	return nil
}

func (s Streamer) stream(ctx context.Context, src runtime.Artifact, dst runtime.Volume) error {
	if !s.p2p.Enabled {
		return s.streamThroughATC(ctx, src, dst)
	}
	p2pSrc, ok := src.(runtime.P2PVolume)
	if !ok {
		return s.streamThroughATC(ctx, src, dst)
	}
	p2pDst, ok := dst.(runtime.P2PVolume)
	if !ok {
		return s.streamThroughATC(ctx, src, dst)
	}

	return s.p2pStream(ctx, p2pSrc, p2pDst)
}

func (s Streamer) streamThroughATC(ctx context.Context, src runtime.Artifact, dst runtime.Volume) error {
	traceAttrs := tracing.Attrs{
		"dest-worker": dst.DBVolume().WorkerName(),
	}
	if srcVolume, ok := src.(runtime.Volume); ok {
		traceAttrs["origin-volume"] = srcVolume.Handle()
		traceAttrs["origin-worker"] = srcVolume.DBVolume().WorkerName()
	}
	_, outSpan := tracing.StartSpan(ctx, "volume.StreamOut", traceAttrs)
	defer outSpan.End()
	out, err := src.StreamOut(ctx, ".", s.compression)

	if err != nil {
		tracing.End(outSpan, err)
		return err
	}

	defer out.Close()

	return dst.StreamIn(ctx, ".", s.compression, out)
}

func (s Streamer) p2pStream(ctx context.Context, src runtime.P2PVolume, dst runtime.P2PVolume) error {
	getCtx, getCancel := context.WithTimeout(ctx, 5*time.Second)
	defer getCancel()

	streamInUrl, err := dst.GetStreamInP2PURL(getCtx, ".")
	if err != nil {
		return err
	}

	_, outSpan := tracing.StartSpan(ctx, "volume.P2pStreamOut", tracing.Attrs{
		"origin-volume": src.Handle(),
		"origin-worker": src.DBVolume().WorkerName(),
		"dest-worker":   dst.DBVolume().WorkerName(),
		"stream-in-url": streamInUrl,
	})
	defer outSpan.End()

	putCtx := ctx
	if s.p2p.Timeout != 0 {
		var putCancel context.CancelFunc
		putCtx, putCancel = context.WithTimeout(putCtx, s.p2p.Timeout)
		defer putCancel()
	}

	return src.StreamP2POut(putCtx, ".", streamInUrl, s.compression)
}

func (s Streamer) StreamFile(ctx context.Context, artifact runtime.Artifact, path string) (io.ReadCloser, error) {
	out, err := artifact.StreamOut(ctx, path, s.compression)
	if err != nil {
		return nil, err
	}

	compressionReader, err := s.compression.NewReader(out)
	if err != nil {
		return nil, err
	}
	tarReader := tar.NewReader(compressionReader)

	_, err = tarReader.Next()
	if err != nil {
		return nil, err
	}

	return fileReadMultiCloser{
		Reader: tarReader,
		closers: []io.Closer{
			out,
			compressionReader,
		},
	}, nil
}

type fileReadMultiCloser struct {
	io.Reader
	closers []io.Closer
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

package worker

import (
	"archive/tar"
	"context"
	"io"
	"time"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/tracing"
	"github.com/hashicorp/go-multierror"
)

type Streamer struct {
	Compression compression.Compression

	EnableP2PStreaming  bool
	P2PStreamingTimeout time.Duration
}

func (s Streamer) Stream(ctx context.Context, srcWorker string, src runtime.Volume, dst runtime.Volume) error {
	if !s.EnableP2PStreaming {
		return s.stream(ctx, srcWorker, src, dst)
	}
	p2pSrc, ok := src.(runtime.P2PVolume)
	if !ok {
		return s.stream(ctx, srcWorker, src, dst)
	}
	p2pDst, ok := dst.(runtime.P2PVolume)
	if !ok {
		return s.stream(ctx, srcWorker, src, dst)
	}

	return s.p2pStream(ctx, srcWorker, p2pSrc, p2pDst)
}

func (s Streamer) stream(ctx context.Context, srcWorker string, src runtime.Volume, dst runtime.Volume) error {
	logger := lagerctx.FromContext(ctx).Session("stream")
	logger.Info("start")
	defer logger.Info("end")

	_, outSpan := tracing.StartSpan(ctx, "volume.StreamOut", tracing.Attrs{
		"origin-volume": src.Handle(),
		"origin-worker": srcWorker,
	})
	defer outSpan.End()
	out, err := src.StreamOut(ctx, ".", s.Compression)

	if err != nil {
		tracing.End(outSpan, err)
		return err
	}

	defer out.Close()

	return dst.StreamIn(ctx, ".", s.Compression, out)
}

func (s Streamer) p2pStream(ctx context.Context, srcWorker string, src runtime.P2PVolume, dst runtime.P2PVolume) error {
	getCtx, getCancel := context.WithTimeout(ctx, 5*time.Second)
	defer getCancel()

	streamInUrl, err := dst.GetStreamInP2PURL(getCtx, ".")
	if err != nil {
		return err
	}

	_, outSpan := tracing.StartSpan(ctx, "volume.P2pStreamOut", tracing.Attrs{
		"origin-volume": src.Handle(),
		"origin-worker": srcWorker,
		"stream-in-url": streamInUrl,
	})
	defer outSpan.End()

	putCtx := ctx
	if s.P2PStreamingTimeout != 0 {
		var putCancel context.CancelFunc
		putCtx, putCancel = context.WithTimeout(putCtx, s.P2PStreamingTimeout)
		defer putCancel()
	}

	return src.StreamP2POut(putCtx, ".", streamInUrl, s.Compression)
}

func (s Streamer) StreamFile(ctx context.Context, volume runtime.Volume, path string) (io.ReadCloser, error) {
	out, err := volume.StreamOut(ctx, path, s.Compression)
	if err != nil {
		return nil, err
	}

	compressionReader, err := s.Compression.NewReader(out)
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

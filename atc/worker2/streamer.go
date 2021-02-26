package worker2

import (
	"archive/tar"
	"context"
	"io"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/tracing"
	"github.com/hashicorp/go-multierror"
)

type Streamer struct {
	Compression compression.Compression
}

func (s Streamer) Stream(ctx context.Context, srcWorker string, src runtime.Volume, dst runtime.Volume) error {
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

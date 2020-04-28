package compression

import (
	"io"

	"github.com/concourse/baggageclaim"
	"github.com/klauspost/compress/zstd"
)

type zstdCompression struct{}

func NewZstdCompression() Compression {
	return &zstdCompression{}
}

func (c *zstdCompression) NewReader(reader io.ReadCloser) (io.ReadCloser, error) {
	d, err := zstd.NewReader(reader)
	if err != nil {
		return nil, err
	}
	return &zstdReader{decoder: d}, nil
}

func (c *zstdCompression) Encoding() baggageclaim.Encoding {
	return baggageclaim.ZstdEncoding
}

type zstdReader struct {
	decoder *zstd.Decoder
}

func (zr *zstdReader) Read(p []byte) (int, error) {
	return zr.decoder.Read(p)
}

func (zr *zstdReader) Close() error {
	zr.decoder.Close()
	return nil
}

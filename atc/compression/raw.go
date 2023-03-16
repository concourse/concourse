package compression

import (
	"io"

	"github.com/concourse/concourse/worker/baggageclaim"
)

type noCompression struct{}

func NewNoCompression() Compression {
	return &noCompression{}
}

func (c *noCompression) NewReader(reader io.ReadCloser) (io.ReadCloser, error) {
	return &rawReader{reader: reader}, nil
}

func (c *noCompression) Encoding() baggageclaim.Encoding {
	return baggageclaim.RawEncoding
}

type rawReader struct {
	reader io.ReadCloser
}

func (zr *rawReader) Read(p []byte) (int, error) {
	return zr.reader.Read(p)
}

func (zr *rawReader) Close() error {
	return zr.reader.Close()
}

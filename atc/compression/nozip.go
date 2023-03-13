package compression

import (
	"io"

	"github.com/concourse/concourse/worker/baggageclaim"
)

type nozipCompression struct{}

func NewNoZipCompression() Compression {
	return &nozipCompression{}
}

func (c *nozipCompression) NewReader(reader io.ReadCloser) (io.ReadCloser, error) {
	return &zozipReader{reader: reader}, nil
}

func (c *nozipCompression) Encoding() baggageclaim.Encoding {
	return baggageclaim.NoZipEncoding
}

type zozipReader struct {
	reader io.ReadCloser
}

func (zr *zozipReader) Read(p []byte) (int, error) {
	return zr.reader.Read(p)
}

func (zr *zozipReader) Close() error {
	return zr.reader.Close()
}

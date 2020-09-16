package compression

import (
	"compress/gzip"
	"io"

	"github.com/concourse/baggageclaim"
)

type gzipCompression struct{}

func NewGzipCompression() Compression {
	return &gzipCompression{}
}

func (c *gzipCompression) NewReader(reader io.ReadCloser) (io.ReadCloser, error) {
	r, err := gzip.NewReader(reader)
	if err != nil {
		return nil, err
	}
	return &gzipReader{reader: r}, nil
}

func (c *gzipCompression) Encoding() baggageclaim.Encoding {
	return baggageclaim.GzipEncoding
}

type gzipReader struct {
	reader *gzip.Reader
}

func (gr *gzipReader) Read(p []byte) (int, error) {
	return gr.reader.Read(p)
}

func (gr *gzipReader) Close() error {
	return gr.reader.Close()
}

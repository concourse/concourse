package compression

import (
	"io"

	"github.com/concourse/concourse/worker/baggageclaim"
	"github.com/klauspost/compress/s2"
)

type s2Compression struct{}

func NewS2Compression() Compression {
	return &s2Compression{}
}

func (c *s2Compression) NewReader(reader io.ReadCloser) (io.ReadCloser, error) {
	d := s2.NewReader(reader)
	return &s2Reader{reader: reader, decoder: d}, nil
}

func (c *s2Compression) Encoding() baggageclaim.Encoding {
	return baggageclaim.S2Encoding
}

type s2Reader struct {
	reader  io.ReadCloser
	decoder *s2.Reader
}

func (sr *s2Reader) Read(p []byte) (int, error) {
	return sr.decoder.Read(p)
}

func (sr *s2Reader) Close() error {
	return sr.reader.Close()
}

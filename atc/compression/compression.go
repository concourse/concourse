package compression

import (
	"io"

	"github.com/concourse/concourse/worker/baggageclaim"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Compression
type Compression interface {
	NewReader(io.ReadCloser) (io.ReadCloser, error)
	Encoding() baggageclaim.Encoding
}

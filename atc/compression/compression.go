package compression

import (
	"io"

	"github.com/concourse/concourse/worker/baggageclaim"
)

//go:generate counterfeiter . Compression

type Compression interface {
	NewReader(io.ReadCloser) (io.ReadCloser, error)
	Encoding() baggageclaim.Encoding
}

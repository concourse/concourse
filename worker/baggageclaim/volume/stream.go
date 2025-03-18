package volume

import (
	"io"

	"github.com/concourse/concourse/worker/baggageclaim/uidgid"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Streamer
type Streamer interface {
	In(io.Reader, string, bool) (bool, error)
	Out(io.Writer, string, bool) error
}

type tarGzipStreamer struct {
	namespacer uidgid.Namespacer
	skipGzip   bool
}

type tarS2Streamer struct {
	namespacer uidgid.Namespacer
}

type tarZstdStreamer struct {
	namespacer uidgid.Namespacer
}

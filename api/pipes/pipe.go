package pipes

import (
	"io"

	"github.com/concourse/atc"
)

type pipe struct {
	resource atc.Pipe

	read  io.ReadCloser
	write io.WriteCloser
}

package pipes

import (
	"io"

	"github.com/concourse/atc/api/resources"
)

type pipe struct {
	resource resources.Pipe

	read  io.ReadCloser
	write io.WriteCloser
}

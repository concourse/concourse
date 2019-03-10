package concourse

import (
	"strings"

	"github.com/concourse/concourse/go-concourse/concourse/internal"
)

var ErrUnauthorized = internal.ErrUnauthorized
var ErrForbidden = internal.ErrForbidden

type PipelineConfigError struct {
	ErrorMessages []string
}

func (pipelineConfigError PipelineConfigError) Error() string {
	return strings.Join(pipelineConfigError.ErrorMessages, "\n")
}

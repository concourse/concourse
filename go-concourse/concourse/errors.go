package concourse

import (
	"errors"
	"strings"

	"github.com/concourse/go-concourse/concourse/internal"
)

var ErrUnauthorized = internal.ErrUnauthorized
var ErrForbidden = internal.ErrForbidden

func NameRequiredError(thing string) error {
	return errors.New(thing + " name required")
}

type PipelineConfigError struct {
	ErrorMessages []string
}

func (pipelineConfigError PipelineConfigError) Error() string {
	return strings.Join(pipelineConfigError.ErrorMessages, "\n")
}

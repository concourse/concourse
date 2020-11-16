package concourse

import (
	"fmt"
	"strings"

	"github.com/concourse/concourse/go-concourse/concourse/internal"
)

// ErrUnauthorized is returned for 401 response codes.
var ErrUnauthorized = internal.ErrUnauthorized

// ErrForbidden is returned for 403 response codes.
var ErrForbidden = internal.ForbiddenError{}

// GenericError is used when no more specific error is available, i.e. a
// generic 500 Internal Server Error response with a message in the body.
type GenericError struct {
	Message string
}

// Error just returns the message from the response body.
func (err GenericError) Error() string {
	return err.Message
}

// InvalidConfigError is returned when saving a pipeline returns errors (i.e.
// validation failures).
type InvalidConfigError struct {
	Errors []string `json:"errors"`
}

// Error lists the errors returned for the config.
func (c InvalidConfigError) Error() string {
	return fmt.Sprintf("invalid pipeline config:\n%s", strings.Join(c.Errors, "\n"))
}

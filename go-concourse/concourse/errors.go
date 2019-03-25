package concourse

import (
	"fmt"
	"strings"

	"github.com/concourse/concourse/go-concourse/concourse/internal"
)

// ErrUnauthorized is returned for 401 response codes.
var ErrUnauthorized = internal.ErrUnauthorized

// ErrForbidden is returned for 403 response codes.
var ErrForbidden = internal.ErrForbidden

// GenericError is used when no more specific error is available, i.e. a
// generic 500 Internal Server Error response with a message in the body.
type GenericError struct {
	Message string
}

// Error just returns the message from the response body.
func (err GenericError) Error() string {
	return err.Message
}

// CommandFailedError is returned when a remotely executed command (e.g. a
// resource check) exited with a nonzero status.
type CommandFailedError struct {
	Command string

	ExitStatus int
	Output     string
}

// Error returns a helpful message showing the command, exit status, and
// output.
func (err CommandFailedError) Error() string {
	return fmt.Sprintf("%s failed with exit status '%d':\n%s\n", err.Command, err.ExitStatus, err.Output)
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

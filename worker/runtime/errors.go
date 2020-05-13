package runtime

import "errors"

// ErrInvalidInput indicates a bad input was supplied.
//
type ErrInvalidInput string

func (e ErrInvalidInput) Error() string {
	return string(e)
}

// ErrNotFound indicates that something wasn't found.
//
type ErrNotFound string

func (e ErrNotFound) Error() string {
	return "not found: " + string(e)
}

var (
	// ErrGracePeriodTimeout indicates that the grace period for a graceful
	// termination has been reached.
	//
	ErrGracePeriodTimeout = errors.New("grace period timeout")

	// ErrNotImplemented indicates that a method is not implemented.
	//
	ErrNotImplemented = errors.New("not implemented")
)

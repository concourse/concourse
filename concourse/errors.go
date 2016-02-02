package concourse

import (
	"errors"

	"github.com/concourse/go-concourse/concourse/internal"
)

var ErrUnauthorized = internal.ErrUnauthorized

func NameRequiredError(thing string) error {
	return errors.New(thing + " name required")
}

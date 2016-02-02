package concourse

import "errors"

func NameRequiredError(thing string) error {
	return errors.New(thing + " name required")
}

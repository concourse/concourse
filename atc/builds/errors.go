package builds

import (
	"errors"
	"fmt"
)

var ErrResourceNotFound = errors.New("resource not found")

type VersionNotFoundError struct {
	Input string
}

func (e VersionNotFoundError) Error() string {
	return fmt.Sprintf("version for input %s not found", e.Input)
}

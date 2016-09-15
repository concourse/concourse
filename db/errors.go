package db

import "errors"

var ErrMultipleContainersFound = errors.New("multiple containers found for given identifier")

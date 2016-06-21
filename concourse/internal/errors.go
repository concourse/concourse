package internal

import (
	"errors"
	"fmt"
)

type UnexpectedResponseError struct {
	error
	StatusCode int
	Status     string
	Body       string
}

func (e UnexpectedResponseError) Error() string {
	return fmt.Sprintf("Unexpected Response\nStatus: %s\nBody:\n%s", e.Status, e.Body)
}

type ResourceNotFoundError struct {
	error
}

func (e ResourceNotFoundError) Error() string {
	return "resource not found"
}

var ErrUnauthorized = errors.New("not authorized")
var ErrForbidden = errors.New("forbidden")

package atcclient

import "fmt"

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
	return fmt.Sprintf("Resource Not Found")
}

package builds

import (
	"fmt"
)

// UnknownResourceError is returned when a 'get' or 'put' step refers to a
// resource which is not in the set of resources provided to the Planner.
type UnknownResourceError struct {
	Resource string
}

func (err UnknownResourceError) Error() string {
	return fmt.Sprintf("unknown resource: %s", err.Resource)
}

// UnknownPrototypeError is returned when a 'run' step refers to a
// prototypes which is not in the set of prototypes provided to the Planner.
type UnknownPrototypeError struct {
	Prototype string
}

func (err UnknownPrototypeError) Error() string {
	return fmt.Sprintf("unknown prototype: %s", err.Prototype)
}

// VersionNotProvidedError is returned when a 'get' step does not have a
// corresponding input provided to the Planner.
type VersionNotProvidedError struct {
	Input string
}

func (err VersionNotProvidedError) Error() string {
	return fmt.Sprintf("version for input %s not provided", err.Input)
}

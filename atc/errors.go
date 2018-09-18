package atc

import "fmt"

type MalformedConfigError struct {
	UnmarshalError error
}

func (malformedConfigError MalformedConfigError) Error() string {
	return fmt.Sprintf("malformed config: %s", malformedConfigError.UnmarshalError.Error())
}

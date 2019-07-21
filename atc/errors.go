package atc

import "fmt"

type MalformedConfigError struct {
	UnmarshalError error
}

func (err MalformedConfigError) Error() string {
	return fmt.Sprintf("malformed config: %s", err.UnmarshalError.Error())
}

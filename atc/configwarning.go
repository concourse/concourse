package atc

import (
	"fmt"
	"regexp"
)

type ConfigWarning struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

var validIdentifiers = regexp.MustCompile(`^\p{L}[\p{L}\d\-.]*$`)

func ValidateIdentifier(identifier string, context ...string) error {
	if identifier != "" && !validIdentifiers.MatchString(identifier) {
		if context != nil {
			return fmt.Errorf("'%s' is not a valid %s identifier", identifier, context)
		}
		return fmt.Errorf("'%s' is not a valid identifier", identifier)
	}
	return nil
}

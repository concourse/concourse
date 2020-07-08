package atc

import (
	"fmt"
	"regexp"
)

type ConfigWarning struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func ValidateIdentifier(identifier string) error {
	if identifier != "" && !regexp.MustCompile(`^\p{L}[\p{L}\d\-.]*$`).MatchString(identifier) {
		return fmt.Errorf("'%s' is not a valid identifier", identifier)
	}
	return nil
}

package atc

import (
	"fmt"
	"regexp"
	"strings"
)

type ConfigWarning struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type InvalidIdentifierError struct {
	Identifier string
	Context    []string
}

func (e *InvalidIdentifierError) Error() string {
	return fmt.Sprintf("'%s' is not a valid identifier", e.Identifier)
}

func (e *InvalidIdentifierError) ConfigWarning() ConfigWarning {
	warning := ConfigWarning{
		Type:    "invalid_identifier",
		Message: e.Error(),
	}

	if e.Context != nil {
		warning.Message = fmt.Sprintf("%s: %s", strings.Join(e.Context, ""), e.Error())
	}

	return warning
}

var validIdentifiers = regexp.MustCompile(`^\p{L}[\p{L}\d\-.]*$`)

func ValidateIdentifier(identifier string, context ...string) error {
	if identifier != "" && !validIdentifiers.MatchString(identifier) {
		return &InvalidIdentifierError{
			Identifier: identifier,
			Context:    context,
		}
	}
	return nil
}

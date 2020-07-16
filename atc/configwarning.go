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

var validIdentifiers = regexp.MustCompile(`^[\p{Ll}\p{Lt}\p{Lm}\p{Lo}][\p{Ll}\p{Lt}\p{Lm}\p{Lo}\d\-.]*$`)

func ValidateIdentifier(identifier string, context ...string) *ConfigWarning {
	if identifier != "" && !validIdentifiers.MatchString(identifier) {
		return &ConfigWarning{
			Type:    "invalid_identifier",
			Message: fmt.Sprintf("%s: %s", strings.Join(context, ""), fmt.Sprintf("'%s' is not a valid identifier", identifier)),
		}
	}
	return nil
}

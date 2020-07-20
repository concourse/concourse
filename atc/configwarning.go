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
var startsWithLetter = regexp.MustCompile(`^[^\p{Ll}\p{Lt}\p{Lm}\p{Lo}]`)
var invalidCharacter = regexp.MustCompile(`([^\p{Ll}\p{Lt}\p{Lm}\p{Lo}\d\-.])`)

func ValidateIdentifier(identifier string, context ...string) *ConfigWarning {
	if identifier != "" && !validIdentifiers.MatchString(identifier) {
		var reason string
		if startsWithLetter.MatchString(identifier) {
			reason = "must start with a lowercase letter"
		} else if invalidChar := invalidCharacter.Find([]byte(identifier[1:])); invalidChar != nil {
			reason = fmt.Sprintf("illegal character '%s'", invalidChar)
		}
		return &ConfigWarning{
			Type:    "invalid_identifier",
			Message: fmt.Sprintf("%s: %s", strings.Join(context, ""), fmt.Sprintf("'%s' is not a valid identifier: %s", identifier, reason)),
		}
	}
	return nil
}

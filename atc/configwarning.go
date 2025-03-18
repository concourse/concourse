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

var validIdentifiers = regexp.MustCompile(`^[\p{Ll}\p{Lt}\p{Lm}\p{Lo}\d][\p{Ll}\p{Lt}\p{Lm}\p{Lo}\d\-_.]*$`)
var startsWithLetterOrNumber = regexp.MustCompile(`^[^\p{Ll}\p{Lt}\p{Lm}\p{Lo}\d]`)
var justNumbers = regexp.MustCompile(`^\d+$`)
var invalidCharacter = regexp.MustCompile(`([^\p{Ll}\p{Lt}\p{Lm}\p{Lo}\d\-_.])`)

func ValidateIdentifier(identifier string, context ...string) (*ConfigWarning, error) {
	if identifier == "" {
		return nil, fmt.Errorf("%s: identifier cannot be an empty string", strings.Join(context, ""))
	}

	// Skip validation for specific contexts
	contextLen := len(context)
	if contextLen >= 2 && (strings.Contains(context[contextLen-1], "set_pipeline") || strings.Contains(context[contextLen-1], "task")) && context[contextLen-2] == ".across" {
		return nil, nil
	}

	// Check if identifier is just numbers (still not allowed)
	if justNumbers.MatchString(identifier) {
		return &ConfigWarning{
			Type:    "invalid_identifier",
			Message: fmt.Sprintf("%s: '%s' is not a valid identifier: cannot be just numbers", strings.Join(context, ""), identifier),
		}, nil
	}

	// Standard identifier validation
	if !validIdentifiers.MatchString(identifier) {
		var reason string

		if startsWithLetterOrNumber.MatchString(identifier) {
			reason = "must start with a lowercase letter or a number"
		} else if invalidChar := invalidCharacter.Find([]byte(identifier[1:])); invalidChar != nil {
			reason = fmt.Sprintf("illegal character '%s'", invalidChar)
		}

		return &ConfigWarning{
			Type:    "invalid_identifier",
			Message: fmt.Sprintf("%s: '%s' is not a valid identifier: %s", strings.Join(context, ""), identifier, reason),
		}, nil
	}

	return nil, nil
}

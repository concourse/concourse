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

var validIdentifiers = regexp.MustCompile(`^[\p{Ll}\p{Lt}\p{Lm}\p{Lo}][\p{Ll}\p{Lt}\p{Lm}\p{Lo}\d\-_.]*$`)
var startsWithLetter = regexp.MustCompile(`^[^\p{Ll}\p{Lt}\p{Lm}\p{Lo}]`)
var invalidCharacter = regexp.MustCompile(`([^\p{Ll}\p{Lt}\p{Lm}\p{Lo}\d\-_.])`)

var numericWithHourSuffix = regexp.MustCompile(`^[0-9]+h$`)
var numericWithMinuteSuffix = regexp.MustCompile(`^[0-9]+m$`)
var numericWithSecondSuffix = regexp.MustCompile(`^[0-9]+s$`)

func ValidateIdentifier(identifier string, context ...string) (*ConfigWarning, error) {
	if identifier == "" {
		return nil, fmt.Errorf("%s: identifier cannot be an empty string", strings.Join(context, ""))
	}

	// Skip validation for specific contexts
	contextLen := len(context)
	if contextLen >= 2 && (strings.Contains(context[contextLen-1], "set_pipeline") || strings.Contains(context[contextLen-1], "task")) && context[contextLen-2] == ".across" {
		return nil, nil
	}

	// Check for valid time formats (only digits followed by 'h', 'm' or 's')
	if numericWithHourSuffix.MatchString(identifier) ||
		numericWithMinuteSuffix.MatchString(identifier) ||
		numericWithSecondSuffix.MatchString(identifier) {
		return nil, nil
	}

	// Standard identifier validation
	if !validIdentifiers.MatchString(identifier) {
		var reason string

		if startsWithLetter.MatchString(identifier) {
			reason = "must start with a lowercase letter"
		} else if invalidChar := invalidCharacter.Find([]byte(identifier[1:])); invalidChar != nil {
			reason = fmt.Sprintf("illegal character '%s'", invalidChar)
		}

		return &ConfigWarning{
			Type:    "invalid_identifier",
			Message: fmt.Sprintf("%s: %s", strings.Join(context, ""), fmt.Sprintf("'%s' is not a valid identifier: %s", identifier, reason)),
		}, nil
	}

	return nil, nil
}

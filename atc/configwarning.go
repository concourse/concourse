package atc

import (
	"fmt"
	"regexp"
	"strings"
)

type ConfigErrors struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (e *ConfigErrors) Error() string {
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

var validIdentifiers = regexp.MustCompile(`^[\p{Ll}\p{Lt}\p{Lm}\p{Lo}\d][\p{Ll}\p{Lt}\p{Lm}\p{Lo}\d\-_.]*$`)
var startsWithLetterOrNumber = regexp.MustCompile(`^[^\p{Ll}\p{Lt}\p{Lm}\p{Lo}\d]`)
var justNumbers = regexp.MustCompile(`^\d+$`)
var invalidCharacter = regexp.MustCompile(`([^\p{Ll}\p{Lt}\p{Lm}\p{Lo}\d\-_.])`)
var placeholderRegex = regexp.MustCompile(`\(\([a-zA-Z0-9_\-\.]+\)\)`)

func ValidateIdentifier(identifier string, context ...string) *ConfigErrors {
	if identifier == "" {
		return &ConfigErrors{
			Type:    "invalid_identifier",
			Message: fmt.Sprintf("%s: identifier cannot be an empty string", strings.Join(context, "")),
		}
	}

	// Skip validation for specific contexts
	contextLen := len(context)
	if contextLen >= 2 && (strings.Contains(context[contextLen-1], "set_pipeline") || strings.Contains(context[contextLen-1], "task")) && context[contextLen-2] == ".across" {
		return nil
	}

	// Check if the identifer contains placeholders and if so, validate the identifier without placeholders
	if placeholderRegex.MatchString(identifier) {
		identifierWithoutPlaceholders := placeholderRegex.ReplaceAllString(identifier, "")

		if strings.HasPrefix(identifier, "-") {
			return &ConfigErrors{
				Type:    "invalid_identifier",
				Message: fmt.Sprintf("%s: '%s' contains invalid characters", strings.Join(context, ""), identifier),
			}
		}
		if justNumbers.MatchString(identifierWithoutPlaceholders) {
			return &ConfigErrors{
				Type:    "invalid_identifier",
				Message: fmt.Sprintf("%s: '%s' identifier cannot be just numbers", strings.Join(context, ""), identifier),
			}
		}
		invalidRegex := regexp.MustCompile(`[^\p{Ll}\p{Lu}\d\-_.]`)
		if invalidRegex.MatchString(identifierWithoutPlaceholders) {
			return &ConfigErrors{
				Type:    "invalid_identifier",
				Message: fmt.Sprintf("%s: '%s' contains invalid characters", strings.Join(context, ""), identifier),
			}
		}

		if !validIdentifiers.MatchString(identifierWithoutPlaceholders) {
			var reason string

			if invalidCharacter.MatchString(identifierWithoutPlaceholders) {
				fmt.Print("pass")
				return &ConfigErrors{
					Type:    "invalid_identifier",
					Message: fmt.Sprintf("%s: '%s' is not a valid identifier: %s", strings.Join(context, ""), identifier, reason),
				}
			} else {
				return nil
			}
		} else {
			return nil
		}
	}

	// Check if identifier is just numbers (still not allowed)
	if justNumbers.MatchString(identifier) {

		return &ConfigErrors{
			Type:    "invalid_identifier",
			Message: fmt.Sprintf("%s: '%s' identifier cannot be just numbers", strings.Join(context, ""), identifier),
		}

	}

	// Standard identifier validation
	if !validIdentifiers.MatchString(identifier) {
		var reason string

		if startsWithLetterOrNumber.MatchString(identifier) {
			reason = "must start with a lowercase letter or a number"
		} else if invalidChar := invalidCharacter.Find([]byte(identifier[1:])); invalidChar != nil {
			reason = fmt.Sprintf("illegal character '%s'", invalidChar)
		}

		return &ConfigErrors{
			Type:    "invalid_identifier",
			Message: fmt.Sprintf("%s: '%s' is not a valid identifier: %s", strings.Join(context, ""), identifier, reason),
		}

	}

	return nil
}

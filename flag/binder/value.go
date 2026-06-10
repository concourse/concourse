package binder

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// flagValue adapts a go-flags-tagged struct field to the pflag.Value
// interface, preserving go-flags conversion and choice-validation
// semantics. The same adapter serves scalars, slices and maps: convert
// dispatches on the destination kind, and Set clears collection defaults
// on the first explicit value just like go-flags' clearReferenceBeforeSet.
type flagValue struct {
	dest         reflect.Value
	choices      []string
	optName      string // go-flags style rendering ("-s, --long"), for error text
	valueName    string // the value-name tag, shown as the usage placeholder
	boolish      bool
	isCollection bool
	defValue     string
	changed      bool
}

func (v *flagValue) Set(s string) error {
	if len(v.choices) > 0 {
		found := false
		if slices.Contains(v.choices, s) {
			found = true
		}

		if !found {
			allowed := strings.Join(v.choices[:len(v.choices)-1], ", ")
			if len(v.choices) > 1 {
				allowed += " or " + v.choices[len(v.choices)-1]
			}

			return fmt.Errorf("Invalid value `%s' for option `%s'. Allowed values are: %s",
				s, v.optName, allowed)
		}
	}

	if v.isCollection && !v.changed {
		// the first explicit value replaces the tag defaults instead of
		// appending to them
		v.dest.Set(emptyValue(v.dest.Type()))
	}
	v.changed = true

	return convert(s, v.dest)
}

func (v *flagValue) String() string {
	return v.defValue
}

func (v *flagValue) Type() string {
	if v.boolish {
		// recognized by pflag's usage rendering so boolean flags print
		// without a value placeholder
		return "bool"
	}
	return v.valueName
}

func emptyValue(tp reflect.Type) reflect.Value {
	if tp.Kind() == reflect.Map {
		return reflect.MakeMap(tp)
	}
	return reflect.Zero(tp)
}

// optString renders the flag the way go-flags' Option.String does, which
// the required-flag and invalid-choice error messages embed.
func optString(short, long string) string {
	if short != "" && long != "" {
		return fmt.Sprintf("-%s, --%s", short, long)
	}
	if short != "" {
		return "-" + short
	}
	return "--" + long
}

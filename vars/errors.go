package vars

import (
	"fmt"
	"strings"
)

type UndefinedVarsError struct {
	Vars []string
}

func (err UndefinedVarsError) Error() string {
	return fmt.Sprintf("undefined vars: %s", strings.Join(err.Vars, ", "))
}

type UnusedVarsError struct {
	Vars []string
}

func (err UnusedVarsError) Error() string {
	return fmt.Sprintf("unused vars: %s", strings.Join(err.Vars, ", "))
}

type MissingFieldError struct {
	Path  string
	Field string
}

func (err MissingFieldError) Error() string {
	return fmt.Sprintf("missing field '%s' in var: %s", err.Field, err.Path)
}

type InvalidFieldError struct {
	Path  string
	Field string
	Value interface{}
}

func (err InvalidFieldError) Error() string {
	return fmt.Sprintf("cannot access field '%s' of non-map value ('%T') from var: %s", err.Field, err.Value, err.Path)
}

type InvalidInterpolationError struct {
	Path  string
	Value interface{}
}

func (err InvalidInterpolationError) Error() string {
	return fmt.Sprintf("cannot interpolate non-primitive value (%T) from var: %s", err.Value, err.Path)
}

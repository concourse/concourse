package flaghelpers

import (
	"fmt"
)

type ParamPairFlag struct {
	Name  string
	Value string
}

func (pair *ParamPairFlag) UnmarshalFlag(value string) error {
	name, value, ok := parseKeyValuePair(value)
	if !ok {
		return fmt.Errorf("invalid input pair '%s' (must be name=value)", value)
	}

	pair.Name = name
	pair.Value = value

	return nil
}

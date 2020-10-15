package flaghelpers

import (
	"fmt"
)

type VariablePairFlag struct {
	Name  string
	Value string
}

func (pair *VariablePairFlag) UnmarshalFlag(value string) error {
	var ok bool
	pair.Name, pair.Value, ok = parseKeyValuePair(value)
	if !ok {
		return fmt.Errorf("invalid variable pair '%s' (must be name=value)", value)
	}

	return nil
}

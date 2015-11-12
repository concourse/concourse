package flaghelpers

import (
	"fmt"
	"strings"
)

type VariablePairFlag struct {
	Name  string
	Value string
}

func (pair *VariablePairFlag) UnmarshalFlag(value string) error {
	vs := strings.SplitN(value, "=", 2)
	if len(vs) != 2 {
		return fmt.Errorf("invalid input pair '%s' (must be name=value)", value)
	}

	pair.Name = vs[0]
	pair.Value = vs[1]

	return nil
}

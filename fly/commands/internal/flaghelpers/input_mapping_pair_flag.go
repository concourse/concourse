package flaghelpers

import (
	"fmt"
	"strings"
)

type InputMappingPairFlag struct {
	Name  string
	Value string
}

func (pair *InputMappingPairFlag) UnmarshalFlag(value string) error {
	var ok bool
	pair.Name, pair.Value, ok = parseKeyValuePair(value)
	if !ok {
		return fmt.Errorf("invalid input mapping '%s' (must be name=value)", value)
	}

	return nil
}

func parseKeyValuePair(value string) (string, string, bool) {
	vs := strings.SplitN(value, "=", 2)
	if len(vs) != 2 {
		return "", "", false
	}
	return vs[0], vs[1], true
}

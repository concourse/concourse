package flaghelpers

import (
	"fmt"
)

type OutputPairFlag struct {
	Name string
	Path string
}

func (pair *OutputPairFlag) UnmarshalFlag(value string) error {
	var ok bool
	pair.Name, pair.Path, ok = parseKeyValuePair(value)
	if !ok {
		return fmt.Errorf("invalid output pair '%s' (must be name=path)", value)
	}

	return nil
}

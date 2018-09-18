package flaghelpers

import (
	"fmt"
	"strings"
)

type OutputPairFlag struct {
	Name string
	Path string
}

func (pair *OutputPairFlag) UnmarshalFlag(value string) error {
	vs := strings.SplitN(value, "=", 2)
	if len(vs) != 2 {
		return fmt.Errorf("invalid output pair '%s' (must be name=path)", value)
	}

	pair.Name = vs[0]
	pair.Path = vs[1]

	return nil
}

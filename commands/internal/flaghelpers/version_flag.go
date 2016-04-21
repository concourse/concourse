package flaghelpers

import (
	"fmt"
	"strings"
)

type VersionFlag struct {
	Key   string
	Value string
}

func (pair *VersionFlag) UnmarshalFlag(value string) error {
	vf := strings.SplitN(value, ":", 2)
	if len(vf) != 2 {
		return fmt.Errorf("invalid version pair '%s' (must be key:value)", value)
	}

	pair.Key = vf[0]
	pair.Value = vf[1]

	return nil
}

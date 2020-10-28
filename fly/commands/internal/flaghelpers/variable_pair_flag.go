package flaghelpers

import (
	"fmt"

	"github.com/concourse/concourse/vars"
)

type VariablePairFlag vars.KVPair

func (pair *VariablePairFlag) UnmarshalFlag(value string) error {
	k, v, ok := parseKeyValuePair(value)
	if !ok {
		return fmt.Errorf("invalid variable pair '%s' (must be name=value)", value)
	}

	var err error
	pair.Ref, err = vars.ParseReference(k)
	if err != nil {
		return err
	}
	pair.Value = v

	return nil
}

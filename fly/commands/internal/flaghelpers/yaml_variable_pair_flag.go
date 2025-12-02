package flaghelpers

import (
	"fmt"

	"github.com/concourse/concourse/vars"
	"github.com/goccy/go-yaml"
)

type YAMLVariablePairFlag vars.KVPair

func (pair *YAMLVariablePairFlag) UnmarshalFlag(value string) error {
	k, v, ok := parseKeyValuePair(value)
	if !ok {
		return fmt.Errorf("invalid variable pair '%s' (must be name=value)", value)
	}

	var err error
	pair.Ref, err = vars.ParseReference(k)
	if err != nil {
		return err
	}
	err = yaml.UnmarshalWithOptions([]byte(v), &pair.Value, yaml.UseJSONUnmarshaler())
	if err != nil {
		return err
	}

	return nil
}

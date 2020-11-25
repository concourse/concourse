package flaghelpers

import (
	"encoding/json"
	"fmt"

	"github.com/concourse/concourse/vars"
	"sigs.k8s.io/yaml"
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
	err = yaml.Unmarshal([]byte(v), &pair.Value, useNumber)
	if err != nil {
		return err
	}

	return nil
}

func useNumber(d *json.Decoder) *json.Decoder {
	d.UseNumber()
	return d
}

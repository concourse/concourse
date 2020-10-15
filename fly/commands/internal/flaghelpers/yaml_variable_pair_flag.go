package flaghelpers

import (
	"fmt"

	yaml "gopkg.in/yaml.v2"
)

type YAMLVariablePairFlag struct {
	Name  string
	Value interface{}
}

func (pair *YAMLVariablePairFlag) UnmarshalFlag(value string) error {
	k, v, ok := parseKeyValuePair(value)
	if !ok {
		return fmt.Errorf("invalid variable pair '%s' (must be name=value)", value)
	}

	pair.Name = k

	err := yaml.Unmarshal([]byte(v), &pair.Value)
	if err != nil {
		return err
	}

	return nil
}

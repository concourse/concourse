package flaghelpers

import (
	"fmt"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

type YAMLVariablePairFlag struct {
	Name  string
	Value interface{}
}

func (pair *YAMLVariablePairFlag) UnmarshalFlag(value string) error {
	vs := strings.SplitN(value, "=", 2)
	if len(vs) != 2 {
		return fmt.Errorf("invalid input pair '%s' (must be name=value)", value)
	}

	pair.Name = vs[0]

	err := yaml.Unmarshal([]byte(vs[1]), &pair.Value)
	if err != nil {
		return err
	}

	return nil
}

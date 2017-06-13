package flaghelpers

import (
	"fmt"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

type VariablePairFlag struct {
	Name  string
	Value interface{}

	OldValue string
}

func (pair *VariablePairFlag) UnmarshalFlag(value string) error {
	vs := strings.SplitN(value, "=", 2)
	if len(vs) != 2 {
		return fmt.Errorf("invalid input pair '%s' (must be name=value)", value)
	}

	pair.Name = vs[0]
	pair.OldValue = vs[1]

	var r interface{}
	err := yaml.Unmarshal([]byte(vs[1]), &r)
	if err != nil {
		return err
	}

	pair.Value = r

	return nil
}

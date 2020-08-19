package flaghelpers

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"strings"
)

type InstanceVarPairFlag struct {
	Name  string
	Value interface{}
}

func (flag *InstanceVarPairFlag) UnmarshalFlag(value string) error {
	vs := strings.SplitN(value, "=", 2)
	if len(vs) != 2 {
		return fmt.Errorf("invalid input flag '%s' (must be name=value)", value)
	}

	flag.Name = vs[0]

	var raw interface{}
	err := yaml.Unmarshal([]byte(vs[1]), &raw)
	if err != nil {
		return err
	}
	flag.Value = raw

	return nil
}

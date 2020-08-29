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

	err := yaml.Unmarshal([]byte(vs[1]), &flag.Value)
	if err != nil {
		return err
	}

	return nil
}

package dummy

import (
	"encoding/json"
	"fmt"
	"strings"

	yaml "sigs.k8s.io/yaml"
)

type VarFlags []VarFlag

// This can go away once we no longer support flags
func (pairs *VarFlags) String() string {
	var fullVarFlags string
	for _, pair := range *pairs {
		value, err := pair.convertToString()
		if err == nil {
			fullVarFlags += ", " + value
		}
	}

	return fullVarFlags
}

// This can go away once we no longer support flags
func (pairs *VarFlags) Set(value string) error {
	varflags := strings.Split(value, ",")
	for _, vs := range varflags {
		var varFlag VarFlag
		err := yaml.Unmarshal([]byte(vs), &varFlag)
		if err != nil {
			return err
		}

		*pairs = append(*pairs, varFlag)
	}

	return nil
}

// XXX: Not sure if this is correct?
// This can go away once we no longer support flags
func (pairs *VarFlags) Type() string {
	return "VarFlags"
}

type VarFlag struct {
	Name  string
	Value interface{}
}

func (pair *VarFlag) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var value string
	err := unmarshal(&value)
	if err != nil {
		return err
	}

	return pair.setString(value)
}

func (pair *VarFlag) MarshalYAML() (interface{}, error) {
	return pair.convertToString()
}

func (pair *VarFlag) convertToString() (string, error) {
	value, err := json.Marshal(pair.Value)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s=%s", pair.Name, value), nil
}

func (pair *VarFlag) setString(value string) error {
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

package flaghelpers

import (
	"errors"
	"gopkg.in/yaml.v2"
	"strings"

	"github.com/concourse/concourse/atc"
)

type InstanceVarPairFlag struct {
	Value atc.DotNotation
}

func (flag *InstanceVarPairFlag) UnmarshalFlag(value string) error {
	flag.Value = atc.DotNotation{}
	for _, v := range strings.Split(value, ",") {
		kv := strings.SplitN(strings.TrimSpace(v), "=", 2)
		if len(kv) == 2 {
			var raw interface{}
			err := yaml.Unmarshal([]byte(kv[1]), &raw)
			if err != nil {
				return err
			}
			flag.Value[kv[0]] = raw
		} else {
			return errors.New("argument format should be <key=value>")
		}
	}
	return nil
}

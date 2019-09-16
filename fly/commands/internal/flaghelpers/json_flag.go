package flaghelpers

import (
	"encoding/json"
)

type JsonFlag struct {
	Raw   string
	Value map[string]string
}

func (v *JsonFlag) UnmarshalFlag(value string) error {
	err := json.Unmarshal([]byte(value), &v.Value)
	if err != nil {
		return err
	}

	v.Raw = value

	return nil
}

package flaghelpers

import (
	"encoding/json"

	"github.com/concourse/concourse/atc"
)

type JsonFlag struct {
	JsonString string
	Version    atc.Version
}

func (v *JsonFlag) UnmarshalFlag(value string) error {
	var version atc.Version
	err := json.Unmarshal([]byte(value), &version)
	if err != nil {
		return err
	}

	v.Version = version
	v.JsonString = value

	return nil
}

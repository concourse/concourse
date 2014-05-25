package config

import "encoding/json"

type Params []byte

func (params *Params) UnmarshalYAML(tag string, data interface{}) error {
	paramsConfig := map[string]interface{}{}

	for k, v := range data.(map[interface{}]interface{}) {
		paramsConfig[k.(string)] = v
	}

	marshalled, err := json.Marshal(paramsConfig)
	if err != nil {
		return err
	}

	*params = marshalled

	return nil
}

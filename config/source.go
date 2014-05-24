package config

import "encoding/json"

type Source []byte

func (source *Source) UnmarshalYAML(tag string, data interface{}) error {
	sourceConfig := map[string]interface{}{}

	for k, v := range data.(map[interface{}]interface{}) {
		sourceConfig[k.(string)] = v
	}

	marshalled, err := json.Marshal(sourceConfig)
	if err != nil {
		return err
	}

	*source = marshalled

	return nil
}

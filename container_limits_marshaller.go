package atc

import (
	"encoding/json"
)

func (c *ContainerLimits) UnmarshalJSON(limit []byte) error {
	var data interface{}

	err := json.Unmarshal(limit, &data)
	if err != nil {
		return err
	}

	climits, err := ContainerLimitsParser(data)
	if err != nil {
		return err
	}

	c.CPU = climits.CPU
	c.Memory = climits.Memory

	return nil
}

func (c *ContainerLimits) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var data interface{}

	err := unmarshal(&data)
	if err != nil {
		return err
	}

	climits, err := ContainerLimitsParser(data)
	if err != nil {
		return err
	}

	c.CPU = climits.CPU
	c.Memory = climits.Memory
	return nil
}

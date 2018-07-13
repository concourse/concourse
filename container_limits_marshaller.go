package atc

import (
	"encoding/json"
	"fmt"
)

func (c *ContainerLimits) UnmarshalJSON(limit []byte) error {

	var data map[interface{}]interface{}

	fmt.Printf("limit: %v", string(limit))
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
	var data map[interface{}]interface{}

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

func (c *ContainerLimits) MarshalYAML() (interface{}, error) {
	return nil, nil
}

func (c *ContainerLimits) MarshalJSON() ([]byte, error) {
	return json.Marshal("")
}

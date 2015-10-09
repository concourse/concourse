package template

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type Variables map[string]string

func (v Variables) Merge(other Variables) Variables {
	merged := Variables{}

	for key, value := range v {
		merged[key] = value
	}

	for key, value := range other {
		merged[key] = value
	}

	return merged
}

func LoadVariablesFromFile(path string) (Variables, error) {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return Variables{}, err
	}

	var variables Variables

	err = yaml.Unmarshal(contents, &variables)
	if err != nil {
		return Variables{}, err
	}

	return variables, nil
}

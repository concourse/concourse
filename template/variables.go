package template

import (
	"fmt"
	"io/ioutil"
	"strings"

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

func LoadVariables(inputs []string) (Variables, error) {
	output := Variables{}

	for _, input := range inputs {
		tokens := strings.SplitN(input, "=", 2)
		if len(tokens) == 1 {
			return Variables{}, fmt.Errorf("input has incorrect format (should be key=value): '%s'", input)
		}
		output[tokens[0]] = tokens[1]
	}

	return output, nil
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

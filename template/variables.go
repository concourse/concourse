package template

import (
	"fmt"
	"strings"
)

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

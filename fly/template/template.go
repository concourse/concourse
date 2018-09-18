package template

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/hashicorp/go-multierror"
)

var templateFormatRegex = regexp.MustCompile(`\{\{([-\w\p{L}]+)\}\}`)

func Present(content []byte) bool {
	return templateFormatRegex.Match(content)
}

func Evaluate(content []byte, variables Variables, allowEmpty bool) ([]byte, error) {
	var variableErrors error

	return templateFormatRegex.ReplaceAllFunc(content, func(match []byte) []byte {
		key := string(templateFormatRegex.FindSubmatch(match)[1])

		value, found := variables[key]
		if !found && !allowEmpty {
			variableErrors = multierror.Append(variableErrors, fmt.Errorf("unbound variable in template: '%s'", key))
			return match
		}

		saveValue, _ := json.Marshal(value)

		return []byte(saveValue)
	}), variableErrors
}

package template

import (
	"encoding/json"
	"fmt"
	"regexp"
)

var templateFormatRegex = regexp.MustCompile(`\{\{([-\w\p{L}]+)\}\}`)

func Evaluate(content []byte, variables Variables) ([]byte, error) {
	var err error

	return templateFormatRegex.ReplaceAllFunc(content, func(match []byte) []byte {
		key := string(templateFormatRegex.FindSubmatch(match)[1])

		value, found := variables[key]
		if !found {
			err = fmt.Errorf("unbound variable in template: '%s'", key)
			return match
		}

		saveValue, _ := json.Marshal(value)

		return []byte(saveValue)
	}), err
}

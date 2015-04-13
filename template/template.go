package template

import (
	"encoding/json"
	"regexp"
)

var templateFormatRegex = regexp.MustCompile(`\{\{([-\w\p{L}]+)\}\}`)

func Evaluate(content []byte, variables Variables) []byte {
	return templateFormatRegex.ReplaceAllFunc(content, func(match []byte) []byte {
		key := string(templateFormatRegex.FindSubmatch(match)[1])

		value, found := variables[key]
		if !found {
			return match
		}

		saveValue, _ := json.Marshal(value)

		return []byte(saveValue)
	})
}

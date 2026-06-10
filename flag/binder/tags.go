package binder

import (
	"strconv"
	"strings"
)

// multiTag holds every value of each key in a raw struct tag.
// reflect.StructTag.Get only returns the first occurrence of a key, but
// go-flags tags such as `default` and `choice` may appear multiple times
// on the same field.
type multiTag map[string][]string

func parseMultiTag(tag string) multiTag {
	m := multiTag{}

	for tag != "" {
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		tag = tag[i:]
		if tag == "" {
			break
		}

		i = 0
		for i < len(tag) && tag[i] > ' ' && tag[i] != ':' && tag[i] != '"' && tag[i] != 0x7f {
			i++
		}
		if i == 0 || i+1 >= len(tag) || tag[i] != ':' || tag[i+1] != '"' {
			break
		}
		name := tag[:i]
		tag = tag[i+1:]

		i = 1
		for i < len(tag) && tag[i] != '"' {
			if tag[i] == '\\' {
				i++
			}
			i++
		}
		if i >= len(tag) {
			break
		}
		value, err := strconv.Unquote(tag[:i+1])
		tag = tag[i+1:]
		if err != nil {
			break
		}

		m[name] = append(m[name], value)
	}

	return m
}

func (m multiTag) Get(key string) string {
	if vals := m[key]; len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func (m multiTag) GetMany(key string) []string {
	return m[key]
}

// truthy mirrors go-flags' !isStringFalsy, used for the `required` and
// `hidden` tags.
func truthy(s string) bool {
	return !(s == "" || s == "false" || s == "no" || s == "0")
}

// envName derives an environment variable name from a flag name the same
// way twentythousandtonnesofcrudeoil did: uppercase, dashes to underscores.
func envName(flagName string) string {
	return strings.ReplaceAll(strings.ToUpper(flagName), "-", "_")
}

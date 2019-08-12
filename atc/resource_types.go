package atc

import (
	"encoding/json"
	"fmt"
	"reflect"
)

type MetadataField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Source map[string]interface{}

func (src Source) MarshalJSON() ([]byte, error) {
	if src == nil {
		return json.Marshal(nil)
	}

	strKeys, err := stringifyKeys(src)
	if err != nil {
		return nil, err
	}

	return json.Marshal(strKeys)
}

type Params map[string]interface{}

func (ps Params) MarshalJSON() ([]byte, error) {
	if ps == nil {
		return json.Marshal(nil)
	}

	strKeys, err := stringifyKeys(ps)
	if err != nil {
		return nil, err
	}

	return json.Marshal(strKeys)
}

type Version map[string]string

func stringifyKeys(root interface{}) (interface{}, error) {
	val := reflect.ValueOf(root)

	switch val.Kind() {
	case reflect.Map:
		sanitized := map[string]interface{}{}

		iter := val.MapRange()
		for iter.Next() {
			k := iter.Key()
			v := iter.Value()

			str, ok := k.Interface().(string)
			if !ok {
				return nil, fmt.Errorf("non-string key: '%s'", k.Interface())
			}

			sub, err := stringifyKeys(v.Interface())
			if err != nil {
				return nil, err
			}

			sanitized[str] = sub
		}

		return sanitized, nil

	case reflect.Slice:
		sanitized := make([]interface{}, val.Len())

		for i := range sanitized {
			v := val.Index(i)

			sub, err := stringifyKeys(v.Interface())
			if err != nil {
				return nil, err
			}

			sanitized[i] = sub
		}

		return sanitized, nil

	default:
		return root, nil
	}
}

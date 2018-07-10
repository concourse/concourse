package atc

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

type Size uint64

const sizeRegex = regexp.MustCompile("[0-9]+(G|M|K|g|m|k)?(b|B)")

func (c *Size) UnmarshalJSON(version []byte) error {

	var data interface{}

	err := json.Unmarshal(version, &data)
	if err != nil {
		return err
	}

	switch actual := data.(type) {
	case string:
		c.Every = actual == "every"
		c.Latest = actual == "latest"
	case map[string]interface{}:
		version := Version{}

		for k, v := range actual {
			if s, ok := v.(string); ok {
				version[k] = strings.TrimSpace(s)
			}
		}

		c.Pinned = version
	default:
		return errors.New("unknown type for version")
	}

	return nil
}

func (c *Size) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var data interface{}

	err := unmarshal(&data)
	if err != nil {
		return err
	}

	switch actual := data.(type) {
	case string:

	case uint64:
	case map[interface{}]interface{}:
		version := Version{}

		for k, v := range actual {
			if ks, ok := k.(string); ok {
				if vs, ok := v.(string); ok {
					version[ks] = strings.TrimSpace(vs)
				}
			}
		}

		c.Pinned = version
	default:
		return errors.New("unknown type for version")
	}

	return nil
}

func (c *Size) MarshalYAML() (interface{}, error) {
	if c.Latest {
		return VersionLatest, nil
	}

	if c.Every {
		return VersionEvery, nil
	}

	if c.Pinned != nil {
		return c.Pinned, nil
	}

	return nil, nil
}

func (c *Size) MarshalJSON() ([]byte, error) {
	if c.Latest {
		return json.Marshal(VersionLatest)
	}

	if c.Every {
		return json.Marshal(VersionEvery)
	}

	if c.Pinned != nil {
		return json.Marshal(c.Pinned)
	}

	return json.Marshal("")
}

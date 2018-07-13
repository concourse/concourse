package atc

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

const VersionLatest = "latest"
const VersionEvery = "every"
const MemoryRegex = "^([0-9]+)([G|M|K|g|m|k]?[b|B])?$"

var VersionConfigDecodeHook = func(
	srcType reflect.Type,
	dstType reflect.Type,
	data interface{},
) (interface{}, error) {
	if dstType != reflect.TypeOf(VersionConfig{}) {
		return data, nil
	}

	switch {
	case srcType.Kind() == reflect.String:
		if s, ok := data.(string); ok {
			return VersionConfig{
				Every:  s == VersionEvery,
				Latest: s == VersionLatest,
			}, nil
		}
	case srcType.Kind() == reflect.Map:
		version := Version{}
		if versionConfig, ok := data.(map[interface{}]interface{}); ok {
			for key, val := range versionConfig {
				if sKey, ok := key.(string); ok {
					if sVal, ok := val.(string); ok {
						version[sKey] = strings.TrimSpace(sVal)
					}
				}
			}

			return VersionConfig{
				Pinned: version,
			}, nil
		}
	}

	return data, nil
}

var ContainerLimitsDecodeHook = func(
	srcType reflect.Type,
	dstType reflect.Type,
	data interface{},
) (interface{}, error) {

	if dstType != reflect.TypeOf(ContainerLimits{}) {
		return data, nil
	}
	var containerLimits ContainerLimits
	if limitsData, ok := data.(map[interface{}]interface{}); ok {
		return ContainerLimitsParser(limitsData)
	}

	return containerLimits, nil
}

var SanitizeDecodeHook = func(
	dataKind reflect.Kind,
	valKind reflect.Kind,
	data interface{},
) (interface{}, error) {
	if valKind == reflect.Map {
		if dataKind == reflect.Map {
			return sanitize(data)
		}
	}

	if valKind == reflect.String {
		if dataKind == reflect.String {
			return data, nil
		}

		if dataKind == reflect.Float64 {
			if f, ok := data.(float64); ok {
				return strconv.FormatFloat(f, 'f', -1, 64), nil
			}

			return nil, errors.New("impossible: float64 != float64")
		}

		// format it as JSON/YAML would
		return json.Marshal(data)
	}

	return data, nil
}

var ContainerLimitsParser = func(data map[interface{}]interface{}) (ContainerLimits, error) {

	var c ContainerLimits
	for key, val := range data {
		if sKey, ok := key.(string); ok {
			if sKey == "memory" {
				if memory, ok := val.(string); ok {
					memory = val.(string)
					memoryBytes, err := parseMemoryLimit(memory)
					if err != nil {
						return ContainerLimits{}, err
					}
					c.Memory = memoryBytes
				} else if memory, ok := val.(int); ok {
					c.Memory = uint64(memory)
				}

			} else if sKey == "cpu" {
				uVal, ok := val.(int)
				if !ok {
					return ContainerLimits{}, errors.New("cpu limit must be an integer")
				}
				c.CPU = uint64(uVal)
			}
		}
	}

	return c, nil
}

func sanitize(root interface{}) (interface{}, error) {
	switch rootVal := root.(type) {
	case map[interface{}]interface{}:
		sanitized := map[string]interface{}{}

		for key, val := range rootVal {
			str, ok := key.(string)
			if !ok {
				return nil, errors.New("non-string key")
			}

			sub, err := sanitize(val)
			if err != nil {
				return nil, err
			}

			sanitized[str] = sub
		}

		return sanitized, nil

	case []interface{}:
		sanitized := make([]interface{}, len(rootVal))
		for i, val := range rootVal {
			sub, err := sanitize(val)
			if err != nil {
				return nil, err
			}
			sanitized[i] = sub
		}
		return sanitized, nil

	default:
		return rootVal, nil
	}
}

func parseMemoryLimit(limit string) (uint64, error) {

	limit = strings.ToUpper(limit)
	var sizeRegex *regexp.Regexp = regexp.MustCompile(MemoryRegex)
	matches := sizeRegex.FindStringSubmatch(limit)

	if len(matches) != 3 {
		return 0, errors.New("could not parse container memory limit")
	}

	value, err := strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		return 0, err
	}

	var power float64
	var base float64 = 2
	var unit string = matches[2]
	switch unit {
	case "KB":
		power = 10
	case "MB":
		power = 20
	case "GB":
		power = 30
	default:
		power = 0
	}

	return value * uint64(math.Pow(base, power)), nil
}

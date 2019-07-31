package atc

import (
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
)

const MemoryRegex = "^([0-9]+)([G|M|K|g|m|k]?[b|B])?$"

func (c *ContainerLimits) UnmarshalJSON(limit []byte) error {
	var data interface{}

	err := json.Unmarshal(limit, &data)
	if err != nil {
		return err
	}

	climits, err := ParseContainerLimits(data)
	if err != nil {
		return err
	}

	c.CPU = climits.CPU
	c.Memory = climits.Memory

	return nil
}

func ParseContainerLimits(data interface{}) (ContainerLimits, error) {
	mapData, ok := data.(map[string]interface{})
	if !ok {
		mapData = make(map[string]interface{})
	}

	var c ContainerLimits

	var memoryBytes uint64
	var uVal int
	var err error

	// the json unmarshaller returns numbers as float64 while yaml returns int
	for key, val := range mapData {
		if key == "memory" {
			switch val.(type) {
			case string:
				memoryBytes, err = parseMemoryLimit(val.(string))
				if err != nil {
					return ContainerLimits{}, err
				}
			case *string:
				if val.(*string) == nil {
					c.Memory = nil
					continue
				}
				memoryBytes, err = parseMemoryLimit(*val.(*string))
				if err != nil {
					return ContainerLimits{}, err
				}
			case float64:
				memoryBytes = uint64(int(val.(float64)))
			case int:
				memoryBytes = uint64(val.(int))
			}
			c.Memory = &memoryBytes

		} else if key == "cpu" {
			switch val.(type) {
			case float64:
				uVal = int(val.(float64))
			case int:
				uVal = val.(int)
			case *int:
				if val.(*int) == nil {
					c.CPU = nil
					continue
				}
				uVal = *val.(*int)
			default:
				return ContainerLimits{}, errors.New("cpu limit must be an integer")
			}
			helper := uint64(uVal)
			c.CPU = &helper

		}
	}

	return c, nil
}

func parseMemoryLimit(limit string) (uint64, error) {
	limit = strings.ToUpper(limit)
	var sizeRegex *regexp.Regexp = regexp.MustCompile(MemoryRegex)
	matches := sizeRegex.FindStringSubmatch(limit)

	if len(matches) > 3 || len(matches) < 1 {
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

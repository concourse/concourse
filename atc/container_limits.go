package atc

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var memoryRegex = regexp.MustCompile(`^([0-9]+)([GMK]?[B])?$`)

type ContainerLimits struct {
	CPU    *CPULimit    `json:"cpu,omitempty"`
	Memory *MemoryLimit `json:"memory,omitempty"`
}

type CPULimit uint64

func (c *CPULimit) UnmarshalJSON(data []byte) error {
	var target float64
	if err := json.Unmarshal(data, &target); err != nil {
		return errors.New("cpu limit must be an integer")
	}
	*c = CPULimit(target)
	return nil
}

type MemoryLimit uint64

func (m *MemoryLimit) UnmarshalJSON(data []byte) error {
	var dst interface{}
	if err := json.Unmarshal(data, &dst); err != nil {
		return err
	}
	switch v := dst.(type) {
	case float64:
		*m = MemoryLimit(v)
	case string:
		var err error
		*m, err = ParseMemoryLimit(v)
		if err != nil {
			return err
		}
	}
	return nil
}

func ParseMemoryLimit(limit string) (MemoryLimit, error) {
	limit = strings.ToUpper(limit)
	matches := memoryRegex.FindStringSubmatch(limit)

	if len(matches) != 3 {
		return 0, errors.New("could not parse container memory limit")
	}

	value, err := strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		return 0, err
	}

	unit := matches[2]
	var power int
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

	return MemoryLimit(value * (1 << power)), nil
}

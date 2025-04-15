package atc

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var memoryRegex = regexp.MustCompile(`(?i)^([0-9]+)(([KMG])(i)?B?)?$`)

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
	var dst any
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
	matches := memoryRegex.FindStringSubmatch(limit)

	if len(matches) == 0 {
		return 0, errors.New("could not parse container memory limit")
	}

	value, err := strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		return 0, err
	}

	// If no unit is specified, return value as is (bytes)
	if len(matches) < 3 || matches[2] == "" {
		return MemoryLimit(value), nil
	}

	// Extract the unit prefix (K, M, G)
	unit := strings.ToUpper(matches[3])

	// All units (KB/MB/GB and KiB/MiB/GiB) are treated as binary units
	var multiplier uint64
	switch unit {
	case "K":
		multiplier = 1 << 10 // 2^10 = 1024
	case "M":
		multiplier = 1 << 20 // 2^20 = 1,048,576
	case "G":
		multiplier = 1 << 30 // 2^30 = 1,073,741,824
	default:
		// Default to bytes for unrecognized units
		return MemoryLimit(value), nil
	}

	return MemoryLimit(value * multiplier), nil
}

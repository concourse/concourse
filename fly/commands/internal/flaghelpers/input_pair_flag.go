package flaghelpers

import (
	"fmt"
	"path/filepath"
	"strings"
)

type InputPairFlag struct {
	Name string
	Path string
}

func (pair *InputPairFlag) UnmarshalFlag(value string) error {
	name, path, ok := parseKeyValuePair(value)
	if !ok {
		return fmt.Errorf("invalid input pair '%s' (must be name=path)", value)
	}

	matches, err := filepath.Glob(path)
	if err != nil {
		return fmt.Errorf("failed to expand path '%s': %s", path, err)
	}

	if len(matches) == 0 {
		return fmt.Errorf("path '%s' does not exist", path)
	}

	if len(matches) > 1 {
		return fmt.Errorf("path '%s' resolves to multiple entries: %s", path, strings.Join(matches, ", "))
	}

	pair.Name = name
	pair.Path = matches[0]

	return nil
}

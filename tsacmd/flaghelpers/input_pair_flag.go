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
	vs := strings.SplitN(value, "=", 2)
	if len(vs) != 2 {
		return fmt.Errorf("invalid input pair '%s' (must be name=path)", value)
	}

	matches, err := filepath.Glob(vs[1])
	if err != nil {
		return fmt.Errorf("failed to expand path '%s': %s", vs[1], err)
	}

	if len(matches) == 0 {
		return fmt.Errorf("path '%s' does not exist", vs[1])
	}

	if len(matches) > 1 {
		return fmt.Errorf("path '%s' resolves to multiple entries: %s", vs[1], strings.Join(matches, ", "))
	}

	pair.Name = vs[0]
	pair.Path = matches[0]

	return nil
}

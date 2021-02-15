package flag

import (
	"fmt"
	"os"
	"path/filepath"
)

type Dir string

func (d *Dir) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var value string
	err := unmarshal(&value)
	if err != nil {
		return err
	}

	return d.Set(value)
}

// Can be removed once flags are deprecated
func (d *Dir) Set(value string) error {
	stat, err := os.Stat(value)
	if err == nil {
		if !stat.IsDir() {
			return fmt.Errorf("path '%s' is not a directory", value)
		}
	}

	abs, err := filepath.Abs(value)
	if err != nil {
		return err
	}

	*d = Dir(abs)

	return nil
}

// Can be removed once flags are deprecated
func (d *Dir) String() string {
	return string(*d)
}

// Can be removed once flags are deprecated
func (d *Dir) Type() string {
	return "Dir"
}

func (d *Dir) Path() string {
	return string(*d)
}


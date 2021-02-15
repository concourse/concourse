package flag

import (
	"fmt"
	"os"
	"path/filepath"
)

type File string

func (f *File) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var value string
	err := unmarshal(&value)
	if err != nil {
		return err
	}

	return f.Set(value)
}

// Can be removed once flags are deprecated
func (f *File) Set(value string) error {
	stat, err := os.Stat(value)
	if err != nil {
		return err
	}

	if stat.IsDir() {
		return fmt.Errorf("path '%s' is a directory, not a file", value)
	}

	abs, err := filepath.Abs(value)
	if err != nil {
		return err
	}

	*f = File(abs)

	return nil
}

// Can be removed once flags are deprecated
func (f *File) String() string {
	return string(*f)
}

// Can be removed once flags are deprecated
func (f *File) Type() string {
	return "File"
}

func (f *File) Path() string {
	return string(*f)
}

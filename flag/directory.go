package flag

import (
	"fmt"
	"os"
	"path/filepath"
)

type Dir string

func (f *Dir) UnmarshalFlag(value string) error {
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

	*f = Dir(abs)

	return nil
}

func (f Dir) Path() string {
	return string(f)
}

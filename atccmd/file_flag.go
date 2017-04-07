package atccmd

import (
	"fmt"
	"os"
	"path/filepath"
)

type FileFlag string

func (f *FileFlag) UnmarshalFlag(value string) error {
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

	*f = FileFlag(abs)

	return nil
}

func (f FileFlag) Path() string {
	return string(f)
}

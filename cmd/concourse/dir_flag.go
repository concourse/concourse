package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type DirFlag string

func (f *DirFlag) UnmarshalFlag(value string) error {
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

	*f = DirFlag(abs)

	return nil
}

func (f DirFlag) Path() string {
	return string(f)
}

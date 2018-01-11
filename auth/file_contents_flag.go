package auth

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
)

type FileContentsFlag string

func (f *FileContentsFlag) UnmarshalFlag(value string) error {
	if value == "" {
		return nil
	}

	matches, err := filepath.Glob(value)
	if err != nil {
		return fmt.Errorf("failed to expand path '%s': %s", value, err)
	}

	if len(matches) == 0 {
		return fmt.Errorf("path '%s' does not exist", value)
	}

	if len(matches) > 1 {
		return fmt.Errorf("path '%s' resolves to multiple entries: %s", value, strings.Join(matches, ", "))
	}

	cert, err := ioutil.ReadFile(matches[0])
	if err != nil {
		return fmt.Errorf("failed to read file from path '%s'", value)
	}

	*f = FileContentsFlag(cert)

	return nil
}

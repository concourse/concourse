//go:build linux || darwin

package volume

import (
	"errors"
	"os"
)

func pathKnown(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}

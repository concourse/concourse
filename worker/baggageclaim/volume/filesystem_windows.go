package volume

import (
	"errors"
	"os"
	"syscall"
)

func pathKnown(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
		return false
	}
	return true
}

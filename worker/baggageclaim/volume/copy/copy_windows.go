package copy

import (
	"fmt"
)

func Cp(followSymlinks bool, src, dest string) error {
	if followSymlinks {
		return fmt.Errorf("FollowSymlinks not supported on Windows")
	}

	return Copy(src, dest)
}

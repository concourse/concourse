//go:build !windows

package interaction

import (
	"io"
	"os"
)

func Stdin() io.Reader {
	return os.Stdin
}

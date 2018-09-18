package ui

import (
	"io"
	"os"
	"runtime"

	colorable "github.com/mattn/go-colorable"
	isatty "github.com/mattn/go-isatty"
)

var Stderr = colorable.NewColorableStderr()

func ForTTY(dst io.Writer) (io.Writer, bool) {
	isTTY := false
	if file, ok := dst.(*os.File); ok && isatty.IsTerminal(file.Fd()) {
		isTTY = true
		if runtime.GOOS == "windows" {
			dst = colorable.NewColorable(file)
		}
	}

	return dst, isTTY
}

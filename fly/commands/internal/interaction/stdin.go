package interaction

import (
	"io"
	"os"

	"golang.org/x/term"
)

// https://github.com/charmbracelet/bubbletea/issues/964
func checkStdin() io.Reader {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return os.Stdin
	}
	return safeReader{Reader: os.Stdin}
}

type safeReader struct {
	io.Reader
}

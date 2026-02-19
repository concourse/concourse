//go:build windows

package pty

import (
	"io"
	"os"

	"golang.org/x/term"
)

func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func OpenRawTerm() (Term, error) {
	return noopRestoreTerm{
		Reader: os.Stdin,
		Writer: os.Stdout,
	}, nil
}

type noopRestoreTerm struct {
	io.Reader
	io.Writer
}

func (noopRestoreTerm) Restore() error { return nil }

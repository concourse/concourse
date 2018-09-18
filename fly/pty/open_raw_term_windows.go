// +build windows

package pty

import (
	"io"
	"os"
)

func IsTerminal() bool {
	return true
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

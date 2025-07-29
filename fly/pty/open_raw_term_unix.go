//go:build !windows
// +build !windows

package pty

import (
	"os"

	"golang.org/x/term"

	pkgterm "github.com/pkg/term"
)

func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func OpenRawTerm() (Term, error) {
	t, err := pkgterm.Open(os.Stdin.Name(), pkgterm.RawMode)
	if err != nil {
		return nil, err
	}

	return t, nil
}

// +build !windows

package pty

import (
	"os"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/pkg/term"
)

func IsTerminal() bool {
	return terminal.IsTerminal(int(os.Stdin.Fd()))
}

func OpenRawTerm() (Term, error) {
	t, err := term.Open(os.Stdin.Name(), term.RawMode)
	if err != nil {
		return nil, err
	}

	return t, nil
}

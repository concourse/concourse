// +build !windows

package pty

import (
	"os"

	"github.com/pkg/term"
)

func OpenRawTerm() (Term, error) {
	t, err := term.Open(os.Stdin.Name(), term.RawMode)
	if err != nil {
		return nil, err
	}

	return t, nil
}

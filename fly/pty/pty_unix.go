// +build !windows

package pty

import (
	"os"

	"github.com/kr/pty"
)

func Open() (PTY, error) {
	p, t, err := pty.Open()
	if err != nil {
		return PTY{}, err
	}

	return PTY{
		TTYR: t,
		TTYW: t,
		PTYR: p,
		PTYW: p,
	}, nil
}

func Getsize(file *os.File) (int, int, error) {
	return pty.Getsize(file)
}

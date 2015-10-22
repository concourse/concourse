// +build !windows

package pty

import (
	"io"
	"os"

	"github.com/kr/pty"
)

func Open() (io.ReadWriteCloser, io.ReadWriteCloser, error) {
	return pty.Open()
}

func Getsize(file *os.File) (int, int, error) {
	return pty.Getsize(file)
}

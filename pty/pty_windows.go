// +build windows

package pty

import (
	"io"
	"os"
)

func Open() (io.ReadWriteCloser, io.ReadWriteCloser, error) {
	r1, w1, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}

	r2, w2, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}

	return rwc{r1, w2}, rwc{r2, w1}, nil
}

func Getsize(*os.File) (int, int, error) {
	return 24, 80, nil
}

type rwc struct {
	io.ReadCloser
	io.WriteCloser
}

func (rwc rwc) Close() error {
	err := rwc.ReadCloser.Close()
	if err != nil {
		return err
	}

	err = rwc.WriteCloser.Close()
	if err != nil {
		return err
	}

	return nil
}

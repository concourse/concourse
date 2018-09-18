// +build windows

package pty

import "os"

func Open() (PTY, error) {
	r1, w1, err := os.Pipe()
	if err != nil {
		return PTY{}, err
	}

	r2, w2, err := os.Pipe()
	if err != nil {
		return PTY{}, err
	}

	return PTY{
		TTYR: r1,
		TTYW: w2,
		PTYR: r2,
		PTYW: w1,
	}, nil
}

func Getsize(*os.File) (int, int, error) {
	return 24, 80, nil
}

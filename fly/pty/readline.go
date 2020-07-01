package pty

import (
	"errors"
	"io"
)

var ErrInterrupted = errors.New("interrupted")

const (
	keyCtrlC     = 3
	keyBackspace = 127
)

func ReadLine(reader io.Reader) ([]byte, error) {
	var buf [1]byte
	var ret []byte

	for {
		n, err := reader.Read(buf[:])
		if n > 0 {
			switch buf[0] {
			case '\b', keyBackspace:
				if len(ret) > 0 {
					ret = ret[:len(ret)-1]
				}
			case '\r', '\n':
				return ret, nil
			case keyCtrlC:
				return nil, ErrInterrupted
			default:
				if isPrintableChar(buf[0]) {
					ret = append(ret, buf[0])
				}
			}
			continue
		}
		if err != nil {
			if err == io.EOF && len(ret) > 0 {
				return ret, nil
			}
			return ret, err
		}
	}
}

func isPrintableChar(b byte) bool {
	return b >= 32
}

package pty

import (
	"context"
	"errors"
	"io"
)

var ErrInterrupted = errors.New("interrupted")

const (
	keyCtrlC     = 3
	keyBackspace = 127
)

func ReadLine(ctx context.Context, reader io.Reader) ([]byte, error) {
	var buf [1]byte
	var ret []byte

	readCh := make(chan readResult)

	for {
		go func() {
			n, err := reader.Read(buf[:])
			readCh <- readResult{n: n, err: err}
		}()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case result := <-readCh:
			n, err := result.n, result.err

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
}

type readResult struct {
	n   int
	err error
}

func isPrintableChar(b byte) bool {
	return b >= 32
}

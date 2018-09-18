package pty

import "os"

type PTY struct {
	TTYR *os.File
	TTYW *os.File
	PTYR *os.File
	PTYW *os.File
}

func (p PTY) Close() error {
	if err := p.TTYR.Close(); err != nil {
		return err
	}

	if err := p.TTYW.Close(); err != nil {
		return err
	}

	if err := p.PTYR.Close(); err != nil {
		return err
	}

	if err := p.PTYW.Close(); err != nil {
		return err
	}

	return nil
}

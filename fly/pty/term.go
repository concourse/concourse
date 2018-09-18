package pty

import "io"

type Term interface {
	io.ReadWriter

	Restore() error
}

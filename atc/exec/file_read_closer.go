package exec

import "io"

type fileReadCloser struct {
	io.Reader
	io.Closer
}

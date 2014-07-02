package limitedstream

import "io"

type Writer struct {
	io.WriteCloser

	Limit int
}

func (w Writer) Write(p []byte) (int, error) {
	wrote := 0

	for wrote < len(p) {
		var chunk []byte
		if wrote+w.Limit >= len(p) {
			chunk = p[wrote:]
		} else {
			chunk = p[wrote:(wrote + w.Limit)]
		}

		n, err := w.WriteCloser.Write(chunk)

		wrote += n

		if err != nil {
			return wrote, err
		}
	}

	return wrote, nil
}

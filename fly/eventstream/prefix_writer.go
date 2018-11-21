package eventstream

import (
	"io"
)

type PrefixWriter struct {
	time    []byte
	writer  io.Writer
	newLine bool
}

func (w *PrefixWriter) Write(b []byte) (int, error) {
	var toWrite []byte

	for _, c := range b {
		if w.newLine {
			toWrite = append(toWrite, w.time...)
		}

		toWrite = append(toWrite, c)

		w.newLine = c == '\n'
	}

	_, err := w.writer.Write(toWrite)
	if err != nil {
		return 0, err
	}

	return len(b), nil
}

func NewPrefixedWriter(prefix string, writer io.Writer) *PrefixWriter {
	return &PrefixWriter{
		time:    []byte(prefix),
		writer:  writer,
		newLine: true,
	}
}

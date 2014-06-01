package utf8stream

import (
	"io"
	"unicode/utf8"
)

type Writer struct {
	destination io.WriteCloser

	dangling []byte
}

func NewWriter(destination io.WriteCloser) *Writer {
	return &Writer{
		destination: destination,
	}
}

func (streamer *Writer) Write(data []byte) (int, error) {
	text := append(streamer.dangling, data...)

	checkEncoding, _ := utf8.DecodeLastRune(text)
	if checkEncoding == utf8.RuneError {
		streamer.dangling = text
		return len(data), nil
	}

	streamer.dangling = nil

	return streamer.destination.Write(text)
}

func (streamer *Writer) Close() error {
	return streamer.destination.Close()
}

package eventstream

import (
	"bytes"
	"io"
	"time"
)

type TimestampedWriter struct {
	showTimestamp bool
	time          []byte
	writer        io.Writer
	newLine       bool
}

func (w *TimestampedWriter) Write(b []byte) (int, error) {
	var toWrite []byte

	for _, c := range b {
		if w.showTimestamp && w.newLine {
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

func NewTimestampedWriter(writer io.Writer, showTime bool) *TimestampedWriter {
	return &TimestampedWriter{
		showTimestamp: showTime,
		writer:        writer,
		newLine:       showTime,
	}
}

func (w *TimestampedWriter) SetTimestamp(time int64) {
	if w.showTimestamp {
		var b bytes.Buffer
		if time != 0 {
			b.WriteString(getUnixTimeAsString(time))
		} else {
			b.WriteString(createEmptyString(8))
		}
		b.WriteString(createEmptyString(2))

		w.time = b.Bytes()
	}
}

func getUnixTimeAsString(timestamp int64) string {
	const posixTimeLayout string = "15:04:05"
	return time.Unix(timestamp, 0).Format(posixTimeLayout)
}

func createEmptyString(length int) string {
	const charset = " "
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[0]
	}
	return string(b)
}

package deadlinewriter

import (
	"io"
	"time"
)

type DeadlineWriter interface {
	io.WriteCloser
	SetDeadline(time.Time) error
}

type TimeoutWriter struct {
	DeadlineWriter
	Timeout time.Duration
}

func (writer TimeoutWriter) Write(d []byte) (int, error) {
	err := writer.DeadlineWriter.SetDeadline(time.Now().Add(writer.Timeout))
	if err != nil {
		return 0, err
	}

	return writer.DeadlineWriter.Write(d)
}

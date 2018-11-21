package eventstream

import (
	"bytes"
	"time"
)

func Timestamped(event EventLog) string {
	return event.FormatLog(event.FormatTimeAsString())
}

func (event EventLog) FormatLog(prefix string) string {
	var b *bytes.Buffer
	b = &bytes.Buffer{}

	writer := NewPrefixedWriter(prefix, b)
	writer.Write([]byte(event.Log))
	return b.String()
}

func (event EventLog) FormatTimeAsString() string {
	var b bytes.Buffer
	if event.Timestamp != 0 {
		b.WriteString(getUnixTimeAsString(event.Timestamp))
	} else {
		b.WriteString(createEmptyString(8))
	}
	b.WriteString(createEmptyString(2))

	return b.String()
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
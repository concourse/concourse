package builder

import (
	"io"
	"strings"
	"unicode/utf8"

	"code.cloudfoundry.org/clock"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
)

func newDBEventWriter(build db.Build, origin event.Origin, clock clock.Clock) io.WriteCloser {
	return &dbEventWriter{
		build:  build,
		origin: origin,
		clock:  clock,
	}
}

type dbEventWriter struct {
	build    db.Build
	origin   event.Origin
	clock    clock.Clock
	dangling []byte
}

func (writer *dbEventWriter) Write(data []byte) (int, error) {
	text := writer.writeDangling(data)
	if text == nil {
		return len(data), nil
	}

	err := writer.saveLog(string(text))
	if err != nil {
		return 0, err
	}

	return len(data), nil
}

func (writer *dbEventWriter) writeDangling(data []byte) []byte {
	text := append(writer.dangling, data...)

	checkEncoding, _ := utf8.DecodeLastRune(text)
	if checkEncoding == utf8.RuneError {
		writer.dangling = text
		return nil
	}

	writer.dangling = nil
	return text
}

func (writer *dbEventWriter) saveLog(text string) error {
	return writer.build.SaveEvent(event.Log{
		Time:    writer.clock.Now().Unix(),
		Payload: text,
		Origin:  writer.origin,
	})
}

func (writer *dbEventWriter) Close() error {
	return nil
}

func newDBEventWriterWithSecretRedaction(build db.Build, origin event.Origin, clock clock.Clock, filter exec.BuildOutputFilter) io.Writer {
	return &dbEventWriterWithSecretRedaction{
		dbEventWriter: dbEventWriter{
			build:  build,
			origin: origin,
			clock:  clock,
		},
		filter: filter,
	}
}

type dbEventWriterWithSecretRedaction struct {
	dbEventWriter
	filter exec.BuildOutputFilter
}

func (writer *dbEventWriterWithSecretRedaction) Write(data []byte) (int, error) {
	var text []byte

	if data != nil {
		text = writer.writeDangling(data)
		if text == nil {
			return len(data), nil
		}
	} else {
		if writer.dangling == nil || len(writer.dangling) == 0 {
			return 0, nil
		}
		text = writer.dangling
	}

	payload := string(text)
	if data != nil {
		idx := strings.LastIndex(payload, "\n")
		if idx >= 0 && idx < len(payload) {
			// Cache content after the last new-line, and proceed contents
			// before the last new-line.
			writer.dangling = ([]byte)(payload[idx+1:])
			payload = payload[:idx+1]
		} else {
			// No new-line found, then cache the log.
			writer.dangling = text
			return len(data), nil
		}
	}

	payload = writer.filter(payload)
	err := writer.saveLog(payload)
	if err != nil {
		return 0, err
	}

	return len(data), nil
}

func (writer *dbEventWriterWithSecretRedaction) Close() error {
	writer.Write(nil)
	return nil
}

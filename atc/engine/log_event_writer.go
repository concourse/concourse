package engine

import (
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"code.cloudfoundry.org/clock"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
)

func newDBEventWriter(build db.Build, origin event.Origin, clock clock.Clock, filter exec.BuildOutputFilter, disableSecretRedaction bool) io.WriteCloser {
	return &dbEventWriter{
		build:                build,
		origin:               origin,
		clock:                clock,
		filter:               filter,
		lastFlush:            clock.Now(),
		disableRedactSecrets: disableSecretRedaction,
	}
}

type dbEventWriter struct {
	build                db.Build
	origin               event.Origin
	clock                clock.Clock
	dangling             []byte
	lastFlush            time.Time
	filter               exec.BuildOutputFilter
	disableRedactSecrets bool
}

func (writer *dbEventWriter) Write(data []byte) (int, error) {
	var text []byte

	if data != nil {
		text = writer.writeDangling(data)
		if text == nil {
			return len(data), nil
		}
	} else {
		if len(writer.dangling) == 0 {
			return 0, nil
		}
		text = writer.dangling
		writer.dangling = nil
	}

	payload := string(text)

	if writer.disableRedactSecrets {
		err := writer.saveLog(payload)
		if err != nil {
			return 0, err
		}

		writer.lastFlush = writer.clock.Now()
		return len(data), nil
	}

	if data != nil {
		idx := strings.LastIndex(payload, "\n")
		if idx < 0 {
			idx = strings.LastIndex(payload, "\r")
		}
		if idx >= 0 && idx < len(payload) {
			// Cache content after the last new-line, and proceed contents
			// before the last new-line.
			writer.dangling = ([]byte)(payload[idx+1:])
			payload = payload[:idx+1]
		} else {
			// Avoid holding onto the buffer indefinitely by flushing if the
			// last flush was more than 1 second ago.
			if writer.clock.Since(writer.lastFlush) < time.Second {
				// No new-line found, then cache the log.
				writer.dangling = text
				return len(data), nil
			}
		}
	}

	payload = writer.filter(payload)
	err := writer.saveLog(payload)
	if err != nil {
		return 0, err
	}

	writer.lastFlush = writer.clock.Now()
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
	writer.Write(nil)
	return nil
}

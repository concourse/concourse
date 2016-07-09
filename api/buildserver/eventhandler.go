package buildserver

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/pivotal-golang/lager"
	"github.com/vito/go-sse/sse"
)

const ProtocolVersionHeader = "X-ATC-Stream-Version"
const CurrentProtocolVersion = "2.0"

func NewEventHandler(logger lager.Logger, buildsDB BuildsDB, buildID int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var start uint = 0
		if r.Header.Get("Last-Event-ID") != "" {
			startString := r.Header.Get("Last-Event-ID")
			_, err := fmt.Sscanf(startString, "%d", &start)
			if err != nil {
				logger.Info("failed-to-parse-last-event-id", lager.Data{"last-event-id": startString})
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			start++
		}

		w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Add(ProtocolVersionHeader, CurrentProtocolVersion)

		writer := eventWriter{
			responseWriter:  w,
			writeFlusher:    nil,
			responseFlusher: w.(http.Flusher),
		}

		w.Header().Add("Vary", "Accept-Encoding")
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			w.Header().Set("Content-Encoding", "gzip")

			gz := gzip.NewWriter(w)
			defer gz.Close()

			writer.responseWriter = gz
			writer.writeFlusher = gz
		}

		events, err := buildsDB.GetBuildEvents(buildID, start)
		if err != nil {
			logger.Error("failed-to-get-build-events", err, lager.Data{"build-id": buildID, "start": start})
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		defer events.Close()

		for {
			logger = logger.WithData(lager.Data{"id": start})

			ev, err := events.Next()
			if err != nil {
				if err == db.ErrEndOfBuildEventStream {
					err := writer.WriteEnd()
					if err != nil {
						logger.Info("failed-to-write-end", lager.Data{"error": err.Error()})
						return
					}
				} else {
					logger.Error("failed-to-get-next-build-event", err)
					return
				}

				return
			}

			err = writer.WriteEvent(start, event.Message{ev})
			if err != nil {
				logger.Info("failed-to-write-event", lager.Data{"error": err.Error()})
				return
			}

			start++
		}
	})
}

type flusher interface {
	Flush() error
}

type eventWriter struct {
	responseWriter  io.Writer
	writeFlusher    flusher
	responseFlusher http.Flusher
}

func (writer eventWriter) WriteEvent(id uint, envelope interface{}) error {
	payload, err := json.Marshal(envelope)
	if err != nil {
		return err
	}

	err = sse.Event{
		ID:   fmt.Sprintf("%d", id),
		Name: "event",
		Data: payload,
	}.Write(writer.responseWriter)
	if err != nil {
		return err
	}

	return writer.flush()
}

func (writer eventWriter) WriteEnd() error {
	err := sse.Event{Name: "end"}.Write(writer.responseWriter)
	if err != nil {
		return err
	}

	return writer.flush()
}

func (writer eventWriter) flush() error {
	if writer.writeFlusher != nil {
		err := writer.writeFlusher.Flush()
		if err != nil {
			return err
		}
	}

	writer.responseFlusher.Flush()

	return nil
}

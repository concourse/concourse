package buildserver

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/vito/go-sse/sse"
)

const ProtocolVersionHeader = "X-ATC-Stream-Version"
const CurrentProtocolVersion = "2.0"

func NewEventHandler(logger lager.Logger, build db.Build) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientNotifier := w.(http.CloseNotifier)

		var eventID uint = 0
		if r.Header.Get("Last-Event-ID") != "" {
			startString := r.Header.Get("Last-Event-ID")
			_, err := fmt.Sscanf(startString, "%d", &eventID)
			if err != nil {
				logger.Info("failed-to-parse-last-event-id", lager.Data{"last-event-id": startString})
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			eventID++
		}

		w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Add("X-Accel-Buffering", "no")
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
			defer db.Close(gz)

			writer.responseWriter = gz
			writer.writeFlusher = gz
		}

		events, err := build.Events(eventID)
		if err != nil {
			logger.Error("failed-to-get-build-events", err, lager.Data{"build-id": build.ID(), "start": eventID})
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		defer db.Close(events)

		for {
			logger = logger.WithData(lager.Data{"id": eventID})

			ev, err := events.Next()
			if err != nil {
				if err == db.ErrEndOfBuildEventStream {
					err := writer.WriteEnd(eventID)
					if err != nil {
						logger.Info("failed-to-write-end", lager.Data{"error": err.Error()})
						return
					}

					<-clientNotifier.CloseNotify()
				} else {
					logger.Error("failed-to-get-next-build-event", err)
					return
				}

				return
			}

			err = writer.WriteEvent(eventID, ev)
			if err != nil {
				logger.Info("failed-to-write-event", lager.Data{"error": err.Error()})
				return
			}

			eventID++
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

func (writer eventWriter) WriteEnd(id uint) error {
	err := sse.Event{ID: fmt.Sprintf("%d", id), Name: "end"}.Write(writer.responseWriter)
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

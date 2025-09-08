package buildserver

import (
	"io"
	"net/http"
	"strconv"

	"code.cloudfoundry.org/lager/v3"
	"github.com/bytedance/sonic"
	"github.com/concourse/concourse/atc/db"
	"github.com/vito/go-sse/sse"
)

const (
	ProtocolVersionHeader  = "X-ATC-Stream-Version"
	CurrentProtocolVersion = "2.0"
)

func NewEventHandler(logger lager.Logger, build db.BuildForAPI) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var eventID uint = 0

		// Parse Last-Event-ID header
		if lastEventID := r.Header.Get("Last-Event-ID"); lastEventID != "" {
			parsedID, err := strconv.ParseUint(lastEventID, 10, 64)
			if err != nil {
				logger.Info("failed-to-parse-last-event-id", lager.Data{"last-event-id": lastEventID})
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			eventID = uint(parsedID) + 1
		}

		// Set response headers
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("X-Accel-Buffering", "no")
		w.Header().Set(ProtocolVersionHeader, CurrentProtocolVersion)

		flusher, ok := w.(http.Flusher)
		if !ok {
			logger.Error("streaming-not-supported", nil)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		writer := eventWriter{
			responseWriter:  w,
			responseFlusher: flusher,
		}

		events, err := build.Events(eventID)
		if err != nil {
			logger.Error("failed-to-get-build-events", err, lager.Data{
				"build-id": build.ID(),
				"start":    eventID,
			})
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer db.Close(events)

		for {
			contextLogger := logger.WithData(lager.Data{"id": eventID})

			ev, err := events.Next()
			if err != nil {
				if err == db.ErrEndOfBuildEventStream {
					if err := writer.WriteEnd(eventID); err != nil {
						contextLogger.Info("failed-to-write-end", lager.Data{"error": err.Error()})
					}
					<-r.Context().Done()
				} else {
					contextLogger.Error("failed-to-get-next-build-event", err)
				}
				return
			}

			if err := writer.WriteEvent(eventID, ev); err != nil {
				contextLogger.Info("failed-to-write-event", lager.Data{"error": err.Error()})
				return
			}

			eventID++
		}
	})
}

type eventWriter struct {
	responseWriter  io.Writer
	responseFlusher http.Flusher
}

func (writer eventWriter) WriteEvent(id uint, envelope any) error {
	payload, err := sonic.Marshal(envelope)
	if err != nil {
		return err
	}

	event := sse.Event{
		ID:   strconv.FormatUint(uint64(id), 10),
		Name: "event",
		Data: payload,
	}

	if err := event.Write(writer.responseWriter); err != nil {
		return err
	}

	writer.responseFlusher.Flush()
	return nil
}

func (writer eventWriter) WriteEnd(id uint) error {
	event := sse.Event{
		ID:   strconv.FormatUint(uint64(id), 10),
		Name: "end",
	}

	if err := event.Write(writer.responseWriter); err != nil {
		return err
	}

	writer.responseFlusher.Flush()
	return nil
}

package buildserver

import (
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/stream"
	"github.com/concourse/concourse/atc/db"
)

const ProtocolVersionHeader = "X-ATC-Stream-Version"
const CurrentProtocolVersion = "2.0"

func NewEventHandler(logger lager.Logger, build db.Build) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		stream.WriteHeaders(w)
		w.Header().Add(ProtocolVersionHeader, CurrentProtocolVersion)

		writer := stream.EventWriter{WriteFlusher: w.(stream.WriteFlusher)}

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

					<-r.Context().Done()
				} else {
					logger.Error("failed-to-get-next-build-event", err)
					return
				}

				return
			}

			err = writer.WriteEvent(eventID, "event", ev)
			if err != nil {
				logger.Info("failed-to-write-event", lager.Data{"error": err.Error()})
				return
			}

			eventID++
		}
	})
}

package buildserver

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/vito/go-sse/sse"
)

const ProtocolVersionHeader = "X-ATC-Stream-Version"
const CurrentProtocolVersion = "2.0"

func NewEventHandler(buildsDB BuildsDB, buildID int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		closed := w.(http.CloseNotifier).CloseNotify()

		w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Add("Connection", "keep-alive")
		w.Header().Add(ProtocolVersionHeader, CurrentProtocolVersion)

		var start uint = 0
		if r.Header.Get("Last-Event-ID") != "" {
			_, err := fmt.Sscanf(r.Header.Get("Last-Event-ID"), "%d", &start)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			start++
		}

		var responseWriter io.Writer = w
		var responseFlusher *gzip.Writer

		w.Header().Add("Vary", "Accept-Encoding")
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			w.Header().Set("Content-Encoding", "gzip")

			gz := gzip.NewWriter(w)
			defer gz.Close()

			responseWriter = gz
			responseFlusher = gz
		}

		events, err := buildsDB.GetBuildEvents(buildID, start)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		defer events.Close()

		es := make(chan atc.Event)
		errs := make(chan error, 1)

		go func() {
			for {
				ev, err := events.Next()
				if err != nil {
					errs <- err
					return
				} else {
					select {
					case es <- ev:
					case <-closed:
						return
					}
				}
			}
		}()

		for {
			select {
			case ev := <-es:
				payload, err := json.Marshal(event.Message{ev})
				if err != nil {
					return
				}

				err = sse.Event{
					ID:   fmt.Sprintf("%d", start),
					Name: "event",
					Data: payload,
				}.Write(responseWriter)
				if err != nil {
					return
				}

				start++

				if responseFlusher != nil {
					err = responseFlusher.Flush()
					if err != nil {
						return
					}
				}

				flusher.Flush()
			case err := <-errs:
				if err == db.ErrEndOfBuildEventStream {
					err = sse.Event{Name: "end"}.Write(responseWriter)
					if err != nil {
						return
					}
				}

				return
			case <-closed:
				return
			}
		}

		return
	})
}

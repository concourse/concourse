package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/event"
	"github.com/vito/go-sse/sse"
)

const ProtocolVersionHeader = "X-ATC-Stream-Version"
const CurrentProtocolVersion = "2.0"

//go:generate counterfeiter . BuildsDB
type BuildsDB interface {
	GetBuild(buildID int) (db.Build, error)
	GetBuildEvents(buildID int) ([]db.BuildEvent, error)
}

type Message struct {
	Event atc.Event
}

type eventEnvelope struct {
	Data    *json.RawMessage `json:"data"`
	Event   atc.EventType    `json:"event"`
	Version atc.EventVersion `json:"version"`
}

func (m Message) MarshalJSON() ([]byte, error) {
	var envelope eventEnvelope

	payload, err := json.Marshal(m.Event)
	if err != nil {
		return nil, err
	}

	envelope.Data = (*json.RawMessage)(&payload)
	envelope.Event = m.Event.EventType()
	envelope.Version = m.Event.Version()

	return json.Marshal(envelope)
}

func (m *Message) UnmarshalJSON(bytes []byte) error {
	var envelope eventEnvelope

	err := json.Unmarshal(bytes, &envelope)
	if err != nil {
		return err
	}

	event, err := event.ParseEvent(envelope.Version, envelope.Event, *envelope.Data)
	if err != nil {
		return err
	}

	m.Event = event

	return nil
}

type EventPayload struct {
	Data    *json.RawMessage `json:"data"`
	Event   atc.EventType    `json:"event"`
	Version atc.EventVersion `json:"version"`
}

func NewHandler(buildsDB BuildsDB, buildID int, eg engine.Engine, censor bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		build, err := buildsDB.GetBuild(buildID)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

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

		if build.Status == db.StatusStarted {
			engineBuild, err := eg.LookupBuild(build)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			events, err := engineBuild.Subscribe(start)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
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
					if censor {
						ev = ev.Censored()
					}

					payload, err := json.Marshal(Message{ev})
					if err != nil {
						return
					}

					err = sse.Event{
						ID:   fmt.Sprintf("%d", start),
						Name: "event",
						Data: payload,
					}.Write(w)
					if err != nil {
						return
					}

					start++

					flusher.Flush()
				case err := <-errs:
					if err == engine.ErrEndOfStream {
						err = sse.Event{Name: "end"}.Write(w)
						if err != nil {
							return
						}
					}

					return
				case <-closed:
					return
				}
			}
		} else {
			events, err := buildsDB.GetBuildEvents(buildID)
			if err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}

			if start >= uint(len(events)) {
				err = sse.Event{Name: "end"}.Write(w)
				if err != nil {
					return
				}

				return
			}

			for _, be := range events[start:] {
				ev, err := event.ParseEvent(atc.EventVersion(be.Version), atc.EventType(be.Type), []byte(be.Payload))
				if err != nil {
					continue
				}

				if censor {
					ev = ev.Censored()
				}

				payload, err := json.Marshal(Message{ev})
				if err != nil {
					return
				}

				err = sse.Event{
					ID:   fmt.Sprintf("%d", start),
					Name: "event",
					Data: payload,
				}.Write(w)
				if err != nil {
					return
				}

				start++

				flusher.Flush()
			}

			err = sse.Event{Name: "end"}.Write(w)
			if err != nil {
				return
			}
		}

		return
	})
}

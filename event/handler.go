package event

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/turbine/event"
	"github.com/vito/go-sse/sse"
)

type BuildsDB interface {
	GetBuild(buildID int) (db.Build, error)
	GetBuildEvents(buildID int) ([]db.BuildEvent, error)
}

type Censor func(event.Event) event.Event

func NewHandler(buildsDB BuildsDB, buildID int, engine engine.Engine, censor Censor) http.Handler {
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
			engineBuild, err := engine.LookupBuild(build)
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

			es := make(chan event.Event)
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
					if censor != nil {
						ev = censor(ev)
					}

					payload, err := json.Marshal(ev)
					if err != nil {
						return
					}

					err = sse.Event{
						ID:   fmt.Sprintf("%d", start),
						Name: string(ev.EventType()),
						Data: payload,
					}.Write(w)
					if err != nil {
						return
					}

					start++

					flusher.Flush()
				case <-errs:
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
				return
			}

			for _, be := range events[start:] {
				ev, err := event.ParseEvent(event.EventType(be.Type), []byte(be.Payload))
				if err != nil {
					continue
				}

				if censor != nil {
					ev = censor(ev)
				}

				payload, err := json.Marshal(ev)
				if err != nil {
					return
				}

				err = sse.Event{
					ID:   fmt.Sprintf("%d", start),
					Name: string(ev.EventType()),
					Data: payload,
				}.Write(w)
				if err != nil {
					return
				}

				start++

				flusher.Flush()
			}
		}

		return
	})
}

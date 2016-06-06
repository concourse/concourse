package buildserver

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/gorilla/websocket"
	"github.com/pivotal-golang/lager"
	"github.com/vito/go-sse/sse"
)

const ProtocolVersionHeader = "X-ATC-Stream-Version"
const CurrentProtocolVersion = "2.0"

func NewEventHandler(logger lager.Logger, buildDB db.BuildDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var flusher http.Flusher
		var responseFlusher *gzip.Writer
		var writer JSONWriter

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

		var events db.EventSource

		if r.Header.Get("Connection") == "Upgrade" && r.Header.Get("Upgrade") == "websocket" {
			var upgrader = websocket.Upgrader{
				HandshakeTimeout: 5 * time.Second,
			}
			w.Header().Add(ProtocolVersionHeader, CurrentProtocolVersion)
			conn, err := upgrader.Upgrade(w, r, w.Header())
			if err != nil {
				logger.Info("unable-to-upgrade-connection-for-websockets", lager.Data{"err": err})
				return
			}
			defer conn.Close()
			writer = WebsocketJSONWriter{conn}

			events, err = buildDB.Events(start)
			if err != nil {
				logger.Error("failed-to-get-build-events", err, lager.Data{"build-id": buildDB.GetID(), "start": start})
				endMessage := websocket.FormatCloseMessage(
					websocket.CloseInternalServerErr,
					"failed-to-get-build-events",
				)
				conn.WriteMessage(websocket.CloseMessage, endMessage)
				return
			}
		} else {
			flusher = w.(http.Flusher)

			w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
			w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Add(ProtocolVersionHeader, CurrentProtocolVersion)

			var responseWriter io.Writer = w

			w.Header().Add("Vary", "Accept-Encoding")
			if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				w.Header().Set("Content-Encoding", "gzip")

				gz := gzip.NewWriter(w)
				defer gz.Close()

				responseWriter = gz
				responseFlusher = gz
			}

			writer = EventSourceJSONWriter{responseWriter}

			var err error
			events, err = buildDB.Events(start)
			if err != nil {
				logger.Error("failed-to-get-build-events", err, lager.Data{"build-id": buildDB.GetID(), "start": start})
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		es := make(chan atc.Event)
		errs := make(chan error, 1)
		closed := make(chan struct{})
		defer close(closed)

		go func() {
			defer events.Close()
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
				err := writer.WriteJSON(WebsocketMessage{
					Type: "event",
					Payload: &WebsocketEvent{
						ID:      int(start),
						Message: event.Message{ev},
					},
				})
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

				if flusher != nil {
					flusher.Flush()
				}
			case err := <-errs:
				if err == db.ErrEndOfBuildEventStream {
					writer.WriteJSON(WebsocketMessage{Type: "end"})
					writer.Close()
				} else {
					logger.Error("failed-to-get-next-build-event", err, lager.Data{"build-id": buildDB.GetID(), "start": start})
					writer.WriteError("failed-to-get-next-build-event")
				}

				return
			}
		}

		return
	})
}

type WebsocketMessage struct {
	Type    string          `json:"type"`
	Payload *WebsocketEvent `json:"payload,omitempty"`
}

type WebsocketEvent struct {
	ID      int           `json:"id"`
	Message event.Message `json:"message"`
}

type JSONWriter interface {
	WriteJSON(WebsocketMessage) error
	WriteError(string) error
	Close() error
}

type WebsocketJSONWriter struct {
	*websocket.Conn
}

type EventSourceJSONWriter struct {
	io.Writer
}

func (w WebsocketJSONWriter) WriteJSON(message WebsocketMessage) error {
	return w.Conn.WriteJSON(message)
}

func (w WebsocketJSONWriter) WriteError(errString string) error {
	endMessage := websocket.FormatCloseMessage(
		websocket.CloseInternalServerErr, errString,
	)
	err := w.WriteMessage(websocket.CloseMessage, endMessage)
	if err != nil {
		return err
	}
	err = w.Conn.Close()
	return err
}

func (w WebsocketJSONWriter) Close() error {
	endMessage := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "end")
	err := w.WriteMessage(websocket.CloseMessage, endMessage)
	if err != nil {
		return err
	}
	err = w.Conn.Close()
	return err
}

func (w EventSourceJSONWriter) WriteJSON(message WebsocketMessage) error {
	switch message.Type {
	case "event":
		payload, err := json.Marshal(message.Payload.Message)
		if err != nil {
			return err
		}

		err = sse.Event{
			ID:   fmt.Sprintf("%d", message.Payload.ID),
			Name: "event",
			Data: payload,
		}.Write(w)
		if err != nil {
			return err
		}
	case "end":
		err := sse.Event{Name: "end"}.Write(w)
		if err != nil {
			return err
		}
	default:
		panic("unexpected WebsocketMessage.Type: " + message.Type)
	}

	return nil
}

func (w EventSourceJSONWriter) WriteError(string) error {
	return nil
}

func (w EventSourceJSONWriter) Close() error {
	return nil
}

package eventstream

import (
	"github.com/concourse/turbine/event"
	"github.com/gorilla/websocket"
)

type EventStream interface {
	NextEvent() (event.Event, error)
}

type WebSocketEventStream struct {
	conn *websocket.Conn
}

func NewWebSocketEventStream(conn *websocket.Conn) *WebSocketEventStream {
	return &WebSocketEventStream{conn: conn}
}

func (s *WebSocketEventStream) NextEvent() (event.Event, error) {
	var msg event.Message
	err := s.conn.ReadJSON(&msg)
	if err != nil {
		return nil, err
	}

	return msg.Event, nil
}

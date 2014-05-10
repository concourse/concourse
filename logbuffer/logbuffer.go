package logbuffer

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type LogBuffer struct {
	content      []byte
	contentMutex *sync.RWMutex

	sinks []*websocket.Conn

	closed bool
}

func NewLogBuffer() *LogBuffer {
	return &LogBuffer{
		contentMutex: new(sync.RWMutex),
	}
}

func (buffer *LogBuffer) Write(data []byte) (int, error) {
	buffer.contentMutex.Lock()

	buffer.content = append(buffer.content, data...)

	newSinks := []*websocket.Conn{}
	for _, sink := range buffer.sinks {
		err := sink.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			continue
		}

		newSinks = append(newSinks, sink)
	}

	buffer.sinks = newSinks

	buffer.contentMutex.Unlock()

	return len(data), nil
}

func (buffer *LogBuffer) Attach(conn *websocket.Conn) {
	buffer.contentMutex.Lock()

	conn.WriteMessage(websocket.TextMessage, buffer.content)

	if buffer.closed {
		closeSink(conn)
	} else {
		buffer.sinks = append(buffer.sinks, conn)
	}

	buffer.contentMutex.Unlock()
}

func (buffer *LogBuffer) Close() {
	buffer.contentMutex.Lock()

	for _, sink := range buffer.sinks {
		closeSink(sink)
	}

	buffer.closed = true
	buffer.sinks = nil

	buffer.contentMutex.Unlock()
}

func (buffer *LogBuffer) Content() []byte {
	buffer.contentMutex.Lock()
	content := make([]byte, len(buffer.content))
	copy(content, buffer.content)
	buffer.contentMutex.Unlock()

	return content
}

func closeSink(sink *websocket.Conn) error {
	err := sink.WriteControl(websocket.CloseMessage, nil, time.Time{})
	if err != nil {
		return err
	}

	return sink.Close()
}

package logfanout

import "encoding/json"

type RawSink struct {
	conn JSONWriteCloser
}

func NewRawSink(conn JSONWriteCloser) Sink {
	return &RawSink{
		conn: conn,
	}
}

func (sink *RawSink) WriteMessage(msg *json.RawMessage) error {
	return sink.conn.WriteJSON(msg)
}

func (sink *RawSink) Close() error {
	return sink.conn.Close()
}

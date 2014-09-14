package logfanout

// interface for the subset of *websocket.Conn that we care about
//
// much easier to test.
type JSONWriteCloser interface {
	WriteJSON(interface{}) error
	Close() error
}

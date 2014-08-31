package eventstream

import (
	"fmt"
	"io"
	"os"

	"github.com/concourse/turbine/event"
	"github.com/gorilla/websocket"
)

var Renderers = map[string]Renderer{
	"1.0": V10Renderer{},
}

type Renderer interface {
	Render(io.Writer, EventStream) int
}

func RenderStream(conn *websocket.Conn) (int, error) {
	var versionMsg event.VersionMessage
	err := conn.ReadJSON(&versionMsg)
	if err != nil {
		return -1, fmt.Errorf("could not determine version: %s", err)
	}

	renderer, found := Renderers[versionMsg.Version]
	if !found {
		return -1, fmt.Errorf("unknown protocol version: %s", versionMsg.Version)
	}

	stream := NewWebSocketEventStream(conn)
	return renderer.Render(os.Stdout, stream), nil
}

package eventstream

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/concourse/turbine/event"
	"github.com/vito/go-sse/sse"
)

var Renderers = map[event.Version]Renderer{
	"1.0": V10Renderer{},
	"1.1": V10Renderer{},
}

type Renderer interface {
	Render(io.Writer, EventStream) int
}

func RenderStream(stream io.Reader) (int, error) {
	reader := sse.NewReader(stream)

	se, err := reader.Next()
	if err != nil {
		return -1, fmt.Errorf("could not determine version: %s", err)
	}

	if se.Name != "version" {
		return -1, fmt.Errorf("expected version event, got %q", se.Name)
	}

	var version event.Version
	err = json.Unmarshal(se.Data, &version)
	if err != nil {
		return -1, fmt.Errorf("malformed version: %s", err)
	}

	renderer, found := Renderers[version]
	if !found {
		return -1, fmt.Errorf("unknown protocol version: %s", version)
	}

	return renderer.Render(os.Stdout, NewSSEEventStream(reader)), nil
}

package eventstream

import (
	"os"

	"github.com/concourse/go-concourse/concourse/eventstream"
	"github.com/vito/go-sse/sse"
)

func RenderStream(eventSource *sse.EventSource) (int, error) {
	return Render(os.Stdout, eventstream.NewSSEEventStream(eventSource)), nil
}

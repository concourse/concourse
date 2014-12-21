package eventstream

import (
	"os"

	"github.com/vito/go-sse/sse"
)

func RenderStream(eventSource *sse.EventSource) (int, error) {
	return Render(os.Stdout, NewSSEEventStream(eventSource)), nil
}

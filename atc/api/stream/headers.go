package stream

import "net/http"

func WriteHeaders(w http.ResponseWriter) {
	w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Add("X-Accel-Buffering", "no")
}

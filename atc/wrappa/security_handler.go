package wrappa

import "net/http"

type SecurityHandler struct {
	XFrameOptions string
	Handler       http.Handler
}

func (handler SecurityHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if handler.XFrameOptions != "" {
		w.Header().Set("X-Frame-Options", handler.XFrameOptions)
	}
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Download-Options", "noopen")
	handler.Handler.ServeHTTP(w, r)
}

package wrappa

import "net/http"

type SecurityHandler struct {
	XFrameOptions           string
	ContentSecurityPolicy   string
	StrictTransportSecurity string
	AdditionalHTTPHeaders   map[string]string
	Handler                 http.Handler
}

func (handler SecurityHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if handler.XFrameOptions != "" {
		w.Header().Set("X-Frame-Options", handler.XFrameOptions)
	}
	if handler.ContentSecurityPolicy != "" {
		w.Header().Set("Content-Security-Policy", handler.ContentSecurityPolicy)
	}
	if handler.StrictTransportSecurity != "" {
		w.Header().Set("Strict-Transport-Security", handler.StrictTransportSecurity)
	}
	for name, value := range handler.AdditionalHTTPHeaders {
		w.Header().Set(name, value)
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Download-Options", "noopen")
	w.Header().Set("Cache-Control", "no-store, private")

	handler.Handler.ServeHTTP(w, r)
}

package wrappa

import "net/http"

type SecurityHandler struct {
	XFrameOptions             string
	ContentSecurityPolicy     string
	StrictTransportSecurity   string
	ReferrerPolicy            string
	CrossOriginOpenerPolicy   string
	CrossOriginResourcePolicy string
	CrossOriginEmbedderPolicy string
	Handler                   http.Handler
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
	if handler.ReferrerPolicy != "" {
		w.Header().Set("Referrer-Policy", handler.ReferrerPolicy)
	}
	if handler.CrossOriginOpenerPolicy != "" {
		w.Header().Set("Cross-Origin-Opener-Policy", handler.CrossOriginOpenerPolicy)
	}
	if handler.CrossOriginResourcePolicy != "" {
		w.Header().Set("Cross-Origin-Resource-Policy", handler.CrossOriginResourcePolicy)
	}
	if handler.CrossOriginEmbedderPolicy != "" {
		w.Header().Set("Cross-Origin-Embedder-Policy", handler.CrossOriginEmbedderPolicy)
	}

	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Download-Options", "noopen")
	w.Header().Set("Cache-Control", "no-store, private")

	handler.Handler.ServeHTTP(w, r)
}

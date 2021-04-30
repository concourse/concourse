package wrappa

import "net/http"

type SecurityHandler struct {
	XFrameOptions         string
	ContentSecurityPolicy string
	Handler               http.Handler
}

func (handler SecurityHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if handler.XFrameOptions != "" {
		w.Header().Set("X-Frame-Options", handler.XFrameOptions)
	}
	if handler.ContentSecurityPolicy != "" {
		w.Header().Set("Content-Security-Policy", handler.ContentSecurityPolicy)
	}
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Download-Options", "noopen")
	w.Header().Set("Cache-Control", "private")
	handler.Handler.ServeHTTP(w, r)
}

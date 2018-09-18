package wrappa

import "net/http"

const concourseVersionHeader = "X-Concourse-Version"

type VersionedHandler struct {
	Version string
	Handler http.Handler
}

func (handler VersionedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(concourseVersionHeader, handler.Version)
	handler.Handler.ServeHTTP(w, r)
}

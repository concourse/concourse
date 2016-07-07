package wrappa

import (
	"net/http"
	"net/url"
)

type redirectingAPIHandler struct {
	externalHost string
}

func RedirectingAPIHandler(
	externalHost string,
) http.Handler {
	return redirectingAPIHandler{
		externalHost: externalHost,
	}
}

func (h redirectingAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u := url.URL{
		Scheme:   "https",
		Host:     h.externalHost,
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}

	http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
}

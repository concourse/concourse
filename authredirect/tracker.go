package authredirect

import (
	"net/http"

	"github.com/gorilla/context"
)

var requestURLKey = "original-request-url"

type Tracker struct {
	http.Handler
}

func (tracker Tracker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	context.Set(r, requestURLKey, r.URL.String())
	tracker.Handler.ServeHTTP(w, r)
}

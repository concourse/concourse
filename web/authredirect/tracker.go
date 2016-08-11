package authredirect

import (
	"context"
	"net/http"
)

var requestURLKey = "original-request-url"

type Tracker struct {
	http.Handler
}

func (tracker Tracker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := context.WithValue(context.Background(), requestURLKey, r.URL.String())
	tracker.Handler.ServeHTTP(w, r.WithContext(ctx))
}

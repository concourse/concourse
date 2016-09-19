package publichandler

import (
	"fmt"
	"net/http"
)

const yearInSeconds = 31536000

func CacheNearlyForever(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", fmt.Sprintf("max-age=%d, private", yearInSeconds))
		handler.ServeHTTP(w, r)
	})
}

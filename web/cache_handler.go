package web

import (
	"fmt"
	"net/http"

	"github.com/klauspost/compress/gzhttp"
)

const yearInSeconds = 31536000

func CacheNearlyForever(handler http.Handler) http.Handler {
	withoutGz := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d, public, immutable", yearInSeconds))
		handler.ServeHTTP(w, r)
	})
	return gzhttp.GzipHandler(withoutGz)
}

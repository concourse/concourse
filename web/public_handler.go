package web

import (
	"io/fs"
	"net/http"
)

func PublicHandler(publicFS fs.FS) http.Handler {
	return CacheNearlyForever(
		http.StripPrefix(
			"/public/",
			http.FileServer(http.FS(publicFS)),
		),
	)
}

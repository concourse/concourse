package web

import (
	"io/fs"
	"mime"
	"net/http"
)

func PublicHandler(publicFS fs.FS) http.Handler {
	mime.AddExtensionType(".woff2", "font/woff2")
	mime.AddExtensionType(".webmanifest", "application/manifest+json")

	return CacheNearlyForever(
		http.StripPrefix(
			"/public/",
			http.FileServer(http.FS(publicFS)),
		),
	)
}

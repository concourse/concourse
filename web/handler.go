package web

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"code.cloudfoundry.org/lager/v3"
)

//go:embed public
var publicEmbed embed.FS

func NewHandler(logger lager.Logger, livePublicDir string) (http.Handler, error) {
	var publicFS fs.FS
	dynamic := livePublicDir != ""

	if dynamic {
		publicFS = os.DirFS(livePublicDir)
	} else {
		var err error
		publicFS, err = fs.Sub(publicEmbed, "public")
		if err != nil {
			return nil, fmt.Errorf("public fs sub: %w", err)
		}
	}

	webMux := http.NewServeMux()

	webMux.Handle("/public/", PublicHandler(publicFS))
	webMux.Handle("/robots.txt", RobotsHandler)
	webMux.Handle("/", IndexHandler(logger, publicFS, dynamic))

	return webMux, nil
}

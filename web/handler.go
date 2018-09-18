package web

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/web/indexhandler"
	"github.com/concourse/web/manifesthandler"
	"github.com/concourse/web/publichandler"
	"github.com/concourse/web/robotshandler"
)

func NewHandler(logger lager.Logger) (http.Handler, error) {

	indexHandler, err := indexhandler.NewHandler(logger)
	if err != nil {
		return nil, err
	}

	publicHandler, err := publichandler.NewHandler()
	if err != nil {
		return nil, err
	}

	manifestHandler := manifesthandler.NewHandler()
	robotsHandler := robotshandler.NewHandler()

	webMux := http.NewServeMux()
	webMux.Handle("/public/", publicHandler)
	webMux.Handle("/manifest.json", manifestHandler)
	webMux.Handle("/robots.txt", robotsHandler)
	webMux.Handle("/", indexHandler)
	return webMux, nil
}

package web

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/v5/web/indexhandler"
	"github.com/concourse/concourse/v5/web/publichandler"
	"github.com/concourse/concourse/v5/web/robotshandler"
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

	robotsHandler := robotshandler.NewHandler()

	webMux := http.NewServeMux()
	webMux.Handle("/public/", publicHandler)
	webMux.Handle("/robots.txt", robotsHandler)
	webMux.Handle("/", indexHandler)
	return webMux, nil
}

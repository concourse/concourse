package callbacks

import (
	"net/http"

	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/callbacks/handler"
	"github.com/concourse/atc/callbacks/routes"
	"github.com/concourse/atc/logfanout"
)

func NewHandler(logger lager.Logger, buildDB handler.BuildDB, logTracker *logfanout.Tracker) (http.Handler, error) {
	builds := handler.NewHandler(logger, buildDB, logTracker)

	handlers := map[string]http.Handler{
		routes.UpdateBuild: http.HandlerFunc(builds.UpdateBuild),

		routes.LogInput: http.HandlerFunc(builds.LogInput),
	}

	return rata.NewRouter(routes.Routes, handlers)
}

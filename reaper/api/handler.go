package api

import (
	"net/http"

	gconn "code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/reaper"
	"github.com/tedsuo/rata"
)

func NewHandler(
	logger lager.Logger,
	gConn gconn.Connection,
) (http.Handler, error) {
	containerServer := NewContainerServer(
		logger.Session("reaper-server"),
		gConn,
	)

	handlers := rata.Handlers{
		reaper.DestroyContainers: http.HandlerFunc(containerServer.DestroyContainers),
	}

	return rata.NewRouter(reaper.Routes, handlers)
}

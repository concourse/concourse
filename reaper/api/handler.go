package api

import (
	"net/http"

	gconn "code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager"
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
		DestroyContainers: http.HandlerFunc(containerServer.DestroyContainers),
		Ping:              http.HandlerFunc(containerServer.Ping),
	}

	return rata.NewRouter(Routes, handlers)
}

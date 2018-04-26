package api

import (
	"net/http"

	"code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/rata"
)

func NewHandler(
	logger lager.Logger,
	gardenClient client.Client,
) (http.Handler, error) {
	containerServer := NewContainerServer(
		logger,
		gardenClient,
	)

	handlers := rata.Handlers{
		DestroyContainers: http.HandlerFunc(containerServer.DestroyContainers),
		Ping:              http.HandlerFunc(containerServer.Ping),
		List:              http.HandlerFunc(containerServer.ListContainers),
	}

	return rata.NewRouter(Routes, handlers)
}

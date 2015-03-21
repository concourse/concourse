package hijackserver

import (
	"net/http"
	"time"

	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
)

type Server struct {
	logger lager.Logger

	workerClient worker.Client

	httpClient *http.Client
}

func NewServer(
	logger lager.Logger,
	workerClient worker.Client,
) *Server {
	return &Server{
		logger:       logger,
		workerClient: workerClient,

		httpClient: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 5 * time.Minute,
			},
		},
	}
}

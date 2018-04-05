package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"code.cloudfoundry.org/garden"
	gconn "code.cloudfoundry.org/garden/client/connection"

	"code.cloudfoundry.org/lager"
)

type ContainerServer struct {
	logger     lager.Logger
	gardenConn gconn.Connection
}

func NewContainerServer(
	logger lager.Logger,
	gConn gconn.Connection,
) *ContainerServer {
	return &ContainerServer{
		gardenConn: gConn,
		logger:     logger,
	}
}

var ErrDestroyContainers = errors.New("failed-to-dstroy")
var ErrPingFailure = errors.New("failed-to-ping-reaper")

func (c *ContainerServer) Ping(w http.ResponseWriter, req *http.Request) {
	hLog := c.logger.Session("ping")
	hLog.Debug("start")
	defer hLog.Debug("done")

	err := c.gardenConn.Ping()
	if err != nil {
		hLog.Error("failed-to-ping-garden-server", err)
		respondWithError(w, ErrPingFailure, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (c *ContainerServer) DestroyContainers(w http.ResponseWriter, req *http.Request) {
	hLog := c.logger.Session("destroy-containers")

	hLog.Debug("start")
	defer hLog.Debug("done")

	w.Header().Set("Content-Type", "application/json")

	var containerHandles []string
	err := json.NewDecoder(req.Body).Decode(&containerHandles)

	if err != nil {
		respondWithError(w, ErrDestroyContainers, http.StatusBadRequest)
		return
	}
	var errExists bool
	for _, containerHandle := range containerHandles {
		err := c.gardenConn.Destroy(containerHandle)
		if err != nil {
			_, ok := err.(garden.ContainerNotFoundError)
			if ok {
				hLog.Info("container-not-found", lager.Data{"handle": containerHandle})
				continue
			}
			hLog.Error("failed-to-delete-container", err, lager.Data{"handle": containerHandle})
			// continue to delete containers even if one fails
			errExists = true
		}
		hLog.Debug("destroyed", lager.Data{"handle": containerHandle})
	}

	if errExists {
		respondWithError(w, ErrDestroyContainers, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	hLog.Info("destroyed")
}

type ErrorResponse struct {
	Message string `json:"error"`
}

func respondWithError(w http.ResponseWriter, err error, statusCode ...int) {
	var code int

	if len(statusCode) > 0 {
		code = statusCode[0]
	} else {
		code = http.StatusInternalServerError
	}

	w.WriteHeader(code)
	errResponse := ErrorResponse{Message: err.Error()}
	json.NewEncoder(w).Encode(errResponse)
}

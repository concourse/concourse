package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/client"

	"code.cloudfoundry.org/lager"
)

type ContainerServer struct {
	logger       lager.Logger
	gardenClient client.Client
}

func NewContainerServer(
	logger lager.Logger,
	gardenClnt client.Client,
) *ContainerServer {
	return &ContainerServer{
		gardenClient: gardenClnt,
		logger:       logger,
	}
}

var ErrDestroyContainers = errors.New("failed-to-destroy")
var ErrListContainers = errors.New("failed-to-list")

var ErrPingFailure = errors.New("failed-to-ping-reaper")

// Ping confirms the server is up and able to talk to the garden server
func (containerServer *ContainerServer) Ping(w http.ResponseWriter, req *http.Request) {
	hLog := containerServer.logger.Session("ping")
	hLog.Debug("start")
	defer hLog.Debug("done")

	err := containerServer.gardenClient.Ping()
	if err != nil {
		hLog.Error("failed-to-ping-garden-server", err)
		respondWithError(w, ErrPingFailure, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ListContainer calls the garden server to request list containers present
func (containerServer *ContainerServer) ListContainers(w http.ResponseWriter, req *http.Request) {
	hLog := containerServer.logger.Session("list-containers")

	hLog.Debug("start")
	defer hLog.Debug("done")

	w.Header().Set("Content-Type", "application/json")

	properties := garden.Properties{}
	containers, err := containerServer.gardenClient.Containers(properties)
	if err != nil {
		hLog.Error("failed-to-list-containers", err)
		respondWithError(w, ErrListContainers, http.StatusInternalServerError)
		return
	}

	containerHandles := []string{}

	for _, container := range containers {
		containerHandles = append(containerHandles, container.Handle())
	}

	err = json.NewEncoder(w).Encode(&containerHandles)
	if err != nil {
		hLog.Error("failed-to-encode-container-handles", err)
		respondWithError(w, ErrListContainers, http.StatusInternalServerError)
		return
	}

	hLog.Info("successfully-listed-containers", lager.Data{"num-handles": len(containerHandles)})
	w.WriteHeader(http.StatusOK)
}

// DestroyContainers calls the garden server to request the removal of each container passed in the body
func (containerServer *ContainerServer) DestroyContainers(w http.ResponseWriter, req *http.Request) {
	hLog := containerServer.logger.Session("destroy-containers")

	hLog.Debug("start")
	defer hLog.Debug("done")

	w.Header().Set("Content-Type", "application/json")

	var containerHandles []string
	err := json.NewDecoder(req.Body).Decode(&containerHandles)
	if err != nil {
		hLog.Error("failed-to-decode-container-handles", err)
		respondWithError(w, ErrDestroyContainers, http.StatusBadRequest)
		return
	}
	var errExists bool
	for _, containerHandle := range containerHandles {
		err := containerServer.gardenClient.Destroy(containerHandle)
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
		hLog.Debug("destroyed-container", lager.Data{"handle": containerHandle})
	}

	if errExists {
		respondWithError(w, ErrDestroyContainers, http.StatusInternalServerError)
		return
	}

	hLog.Info("successfully-destroyed-containers", lager.Data{"num-handles": len(containerHandles)})
	w.WriteHeader(http.StatusNoContent)
	return
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

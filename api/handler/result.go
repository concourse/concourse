package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	ProleBuilds "github.com/winston-ci/prole/api/builds"

	"github.com/winston-ci/winston/builds"
)

func (handler *Handler) SetResult(w http.ResponseWriter, r *http.Request) {
	job := r.FormValue(":job")
	idStr := r.FormValue(":build")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var build ProleBuilds.Build
	if err := json.NewDecoder(r.Body).Decode(&build); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Printf("handling result: %#v\n", build)

	var state builds.BuildState

	switch build.Status {
	case "failed":
		state = builds.BuildStateFailed
	case "succeeded":
		state = builds.BuildStateSucceeded
	case "errored":
		state = builds.BuildStateErrored
	default:
		log.Println("unknown status:", build.Status)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Printf("saving state: %#v\n", state)

	_, err = handler.db.SaveBuildState(job, id, state)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

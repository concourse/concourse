package handler

import (
	"encoding/json"
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

	var state builds.BuildState

	switch build.Status {
	case "failed":
		state = builds.BuildStateFailed
	case "succeeded":
		state = builds.BuildStateSucceeded
	}

	_, err = handler.db.SaveBuildState(job, id, state)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

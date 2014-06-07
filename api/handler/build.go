package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	ProleBuilds "github.com/winston-ci/prole/api/builds"

	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
)

func (handler *Handler) UpdateBuild(w http.ResponseWriter, r *http.Request) {
	job := r.FormValue(":job")
	idStr := r.FormValue(":build")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	build, err := handler.db.GetBuild(job, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var proleBuild ProleBuilds.Build
	if err := json.NewDecoder(r.Body).Decode(&proleBuild); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Printf("updating build: %#v\n", proleBuild)

	var status builds.Status

	switch proleBuild.Status {
	case ProleBuilds.StatusStarted:
		status = builds.StatusStarted
	case ProleBuilds.StatusSucceeded:
		status = builds.StatusSucceeded
	case ProleBuilds.StatusFailed:
		status = builds.StatusFailed
	case ProleBuilds.StatusErrored:
		if build.Status == builds.StatusAborted {
			status = builds.StatusAborted
		} else {
			status = builds.StatusErrored
		}
	default:
		log.Println("unknown status:", proleBuild.Status)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Printf("saving status: %#v\n", status)

	err = handler.db.SaveBuildStatus(job, id, status)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	switch proleBuild.Status {
	case ProleBuilds.StatusStarted:
		for _, input := range proleBuild.Inputs {
			err := handler.db.SaveCurrentVersion(job, input.Name, builds.Version(input.Version))
			if err != nil {
				log.Println("error saving source:", err)
				w.WriteHeader(http.StatusInternalServerError)
			}

			err = handler.db.SaveBuildInput(job, id, buildInputFrom(input))
			if err != nil {
				log.Println("error saving input:", err)
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
	case ProleBuilds.StatusSucceeded:
		for _, output := range proleBuild.Outputs {
			err := handler.db.SaveCurrentVersion(job, output.Name, builds.Version(output.Version))
			if err != nil {
				log.Println("error saving source:", err)
				w.WriteHeader(http.StatusInternalServerError)
			}

			err = handler.db.SaveOutputVersion(job, id, output.Name, builds.Version(output.Version))
			if err != nil {
				log.Println("error saving source:", err)
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

func buildInputFrom(input ProleBuilds.Input) builds.Input {
	metadata := make([]builds.MetadataField, len(input.Metadata))
	for i, md := range input.Metadata {
		metadata[i] = builds.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	return builds.Input{
		Name:     input.Name,
		Source:   config.Source(input.Source),
		Version:  builds.Version(input.Version),
		Metadata: metadata,
	}
}
